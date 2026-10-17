package service

import (
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowcatalog"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/types"
	"os"
	"path/filepath"
	"strings"
)

// Local approved component metadata, populated from saved Agent resources.
func pageStarterGraph(c langflowcatalog.Catalog, cfg types.CustomAgentConfig, scripts []langflowdef.StructuredScript) (json.RawMessage, error) {
	root := os.Getenv("TEKNOWRA_LANGFLOW_COMPONENTS_ROOT")
	if root == "" {
		root = "integrations/langflow"
	}
	raw, err := os.ReadFile(filepath.Join(root, "templates/agent-components.json"))
	if err != nil || len(raw) > 2*1024*1024 {
		return nil, fmt.Errorf("通用流程模板不可用")
	}
	return buildPageStarterGraph(raw, c, cfg, scripts)
}
func buildPageStarterGraph(raw []byte, c langflowcatalog.Catalog, cfg types.CustomAgentConfig, scripts []langflowdef.StructuredScript) (json.RawMessage, error) {
	var templates struct {
		Nodes map[string]map[string]any `json:"nodes"`
		Edges map[string]map[string]any `json:"edges"`
	}
	if json.Unmarshal(raw, &templates) != nil {
		return nil, fmt.Errorf("流程模板无效")
	}
	nodes := []any{}
	edges := []any{}
	agentID := "Agent-platform"
	outputID := "ChatOutput-platform"
	rewrite := func(source map[string]any, replacements map[string]string) map[string]any {
		bytes, _ := json.Marshal(source)
		var obj map[string]any
		_ = json.Unmarshal(bytes, &obj)
		var visit func(any) any
		visit = func(v any) any {
			switch x := v.(type) {
			case string:
				for old, newID := range replacements {
					x = strings.ReplaceAll(x, old, newID)
				}
				return x
			case map[string]any:
				for k, item := range x {
					x[k] = visit(item)
				}
			case []any:
				for i, item := range x {
					x[i] = visit(item)
				}
			}
			return v
		}
		visit(obj)
		return obj
	}
	add := func(kind, id, field, value string, index int) error {
		source, ok := templates.Nodes[kind]
		if !ok {
			return fmt.Errorf("模板缺少组件")
		}
		old, _ := source["id"].(string)
		n := rewrite(source, map[string]string{old: id})
		n["position"] = map[string]any{"x": index % 3 * 420, "y": index / 3 * 420}
		n["selected"] = false
		data := n["data"].(map[string]any)
		meta := data["node"].(map[string]any)
		t := meta["template"].(map[string]any)
		for _, key := range []string{"directory_connection", "bridge_url", "task_capability", "node_key", "tools_json"} {
			if f, ok := t[key].(map[string]any); ok {
				f["value"] = ""
			}
		}
		if f, ok := t["directory_agent_id"].(map[string]any); ok {
			f["value"] = c.AgentID
		}
		if field != "" {
			f := t[field].(map[string]any)
			f["value"] = value
			f["options"] = []string{value}
		}
		if kind == "Agent" {
			meta["display_name"] = "业务智能体"
			t["system_prompt"].(map[string]any)["value"] = cfg.SystemPrompt
		}
		if kind == "ChatInput" {
			t["input_value"].(map[string]any)["value"] = ""
		}
		if kind == "TeKnowraSkills" {
			enabled := false
			for _, p := range scripts {
				if p.Skill == strings.Split(value, " · ")[len(strings.Split(value, " · "))-1] {
					enabled = true
				}
			}
			t["enable_scripts"].(map[string]any)["value"] = enabled
		}
		nodes = append(nodes, n)
		if kind == "ChatOutput" {
			return nil
		}
		edge, ok := templates.Edges[kind]
		if !ok {
			return fmt.Errorf("模板缺少连接")
		}
		src, _ := edge["source"].(string)
		dst, _ := edge["target"].(string)
		target := agentID
		if kind == "Agent" {
			target = outputID
		}
		e := rewrite(edge, map[string]string{src: id, dst: target})
		e["id"] = "edge-" + id + "-" + target
		edges = append(edges, e)
		return nil
	}
	if err := add("ChatInput", "ChatInput-platform", "", "", 0); err != nil {
		return nil, err
	}
	if err := add("Agent", agentID, "", "", 1); err != nil {
		return nil, err
	}
	if err := add("ChatOutput", outputID, "", "", 2); err != nil {
		return nil, err
	}
	index := 3
	for _, r := range c.Models {
		if err := add("TeKnowraModel", "TeKnowraModel-platform", "model_id", r.Name+" · "+r.ID, index); err != nil {
			return nil, err
		}
		index++
		break
	}
	for i, r := range c.KnowledgeBases {
		if err := add("TeKnowraKnowledge", fmt.Sprintf("TeKnowraKnowledge-platform-%d", i), "knowledge_base_id", r.Name+" · "+r.ID, index); err != nil {
			return nil, err
		}
		index++
	}
	for i, r := range c.Services {
		if err := add("TeKnowraMCP", fmt.Sprintf("TeKnowraMCP-platform-%d", i), "service_id", r.Name+" · "+r.ID, index); err != nil {
			return nil, err
		}
		index++
	}
	for i, r := range c.Skills {
		if err := add("TeKnowraSkills", fmt.Sprintf("TeKnowraSkills-platform-%d", i), "skill_name", r.Name+" · "+r.ID, index); err != nil {
			return nil, err
		}
		index++
	}
	return json.Marshal(map[string]any{"nodes": nodes, "edges": edges, "viewport": map[string]any{"x": 0, "y": 0, "zoom": 0.65}})
}
