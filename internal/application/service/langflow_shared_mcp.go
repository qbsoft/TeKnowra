package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

func (s *agentService) prepareLangflowSharedMCP(ctx context.Context, b langflowdef.Binding, c *types.AgentConfig, metadata interfaces.MCPMetadataService, bus *event.EventBus, sessionID, messageID string, check func(context.Context) error) error {
	if !c.SharedAgentReadOnly {
		return nil
	}
	seen := map[string]bool{}
	for _, binding := range b.Tools {
		id := binding.ServiceID
		if seen[id] {
			continue
		}
		seen[id] = true
		var required []string
		for _, tool := range b.Tools {
			if tool.ServiceID == id {
				required = append(required, tool.Name)
			}
		}
		load := func(current context.Context) error {
			if err := check(current); err != nil {
				return err
			}
			svc, err := s.mcpServiceService.GetMCPServiceByID(current, b.WorkspaceID, id)
			if err != nil || svc == nil || !svc.Enabled || (!svc.IsBuiltin && svc.TenantID != b.WorkspaceID) {
				return fmt.Errorf("共享MCP服务不可用")
			}
			registry := tools.NewToolRegistry()
			defer registry.Cleanup(current)
			lookup := func(next context.Context, tenant uint64, service string) (*types.MCPService, error) {
				if err := check(next); err != nil {
					return nil, err
				}
				return s.mcpServiceService.GetMCPServiceByID(next, tenant, service)
			}
			if _, err = tools.RegisterMCPTools(current, registry, []*types.MCPService{svc}, s.mcpManager, s.toolApprovalGate, c.MCPAuthWaitTimeout, lookup, &tools.MCPMetadataIO{Get: metadata.GetMCPMetadata, Put: metadata.PersistMCPMetadata}); err != nil {
				return err
			}
			registry.BindSession(sessionID)
			args, _ := json.Marshal(map[string]any{"mode": "list_tools", "server_id": id, "refresh": true, "limit": 1})
			principal := types.MCPOAuthPrincipalFromContext(current)
			operation := tools.WithToolExecContext(current, &tools.ToolExecContext{SessionID: sessionID, AssistantMessageID: messageID, ToolCallID: "langflow-directory-" + id, UserID: principal.StorageID(), EventBus: bus, ApprovalCtx: current})
			result, err := registry.ExecuteTool(operation, tools.ToolDiscoverMCPTools, args)
			if err != nil {
				return err
			}
			if result == nil || !result.Success {
				return fmt.Errorf("共享MCP目录授权或同步未完成")
			}
			return check(current)
		}
		if err := ensureLangflowCallerDirectory(ctx, metadata, b.WorkspaceID, id, load, check, required...); err != nil {
			return err
		}
	}
	return nil
}
