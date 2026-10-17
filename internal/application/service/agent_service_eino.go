package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/agent/einoruntime"
	"github.com/Tencent/WeKnora/internal/agent/langflowverify"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/models/rerank"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"os"
	"strings"
)

func (s *agentService) createEinoEngine(ctx context.Context, c *types.AgentConfig, m chat.Chat, rank rerank.Reranker, bus *event.EventBus, sessionID string) (interfaces.AgentEngine, error) {
	// Keep bindings and history intact. Removing a binding would silently fall
	// back to the native engine instead of retiring the old execution entry.
	if os.Getenv("TEKNOWRA_EINO_DISABLED") == "true" {
		return nil, fmt.Errorf("Eino执行已停用，请使用Langflow智能体；历史记录保留")
	}
	if c == nil {
		return nil, fmt.Errorf("Agent配置缺失")
	}
	id, bound := s.cfg.EinoDefinitionForAgent(c)
	d, published := einodef.Lookup(id)
	if !bound || !published {
		return nil, fmt.Errorf("Eino定义尚未发布或绑定")
	}
	if err := validateEinoDefinitionConfig(d, c); err != nil {
		return nil, err
	}
	if len(d.ReadOnlyTools) > 0 && !d.Knowledge {
		return s.createEinoReadOnlyEngine(ctx, d, c, m, bus)
	}
	retrieve := func(ctx context.Context, scope types.SearchTargets, queries []string) ([]*types.SearchResult, error) {
		local := *c
		local.SearchTargets = cloneEinoTargets(scope)
		// The knowledge capability never registers the sibling MCP capability.
		local.MCPSelectionMode = "none"
		local.MCPServices = nil
		local.AllowedTools = []string{tools.ToolSearchKnowledge}
		registry := tools.NewToolRegistry()
		defer registry.Cleanup(ctx)
		if err := s.registerTools(ctx, registry, &local, rank, m, sessionID); err != nil {
			return nil, err
		}
		args, _ := json.Marshal(map[string]any{"query": strings.Join(queries, "；")})
		result, err := registry.ExecuteTool(ctx, tools.ToolSearchKnowledge, args)
		if err != nil {
			return nil, err
		}
		if result == nil || !result.Success {
			return nil, fmt.Errorf("authorized knowledge retrieval failed")
		}
		raw, err := json.Marshal(result.Data["results"])
		if err != nil {
			return nil, err
		}
		var rows []map[string]any
		if err = json.Unmarshal(raw, &rows); err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row["chunk_id"] != nil {
				row["id"] = row["chunk_id"]
			} else {
				row["id"] = row["faq_id"]
			}
		}
		raw, err = json.Marshal(rows)
		if err != nil {
			return nil, err
		}
		var refs []*types.SearchResult
		err = json.Unmarshal(raw, &refs)
		return refs, err
	}
	o := einoruntime.Options{Definition: d.ID, Config: c, Model: m, Events: bus, Revalidate: c.EinoRevalidate, Retrieve: retrieve, FilterEvidence: c.EinoFilterEvidence}
	if len(d.ReadOnlyTools) > 0 {
		var err error
		o, err = s.buildEinoReadOnlyOptions(ctx, d, c, m, bus)
		if err != nil {
			return nil, err
		}
		o.Retrieve = retrieve
	}
	// Local throwaway spike: reuse capability preparation, execute a Langflow
	// graph only for the dedicated opt-in verification Agent. Existing Eino
	// publications and their executors are unchanged.
	if langflowverify.Enabled(c.AgentID) {
		return langflowverify.New(o)
	}
	return einoruntime.New(ctx, o)
}
