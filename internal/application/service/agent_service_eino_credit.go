package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/agent/einoruntime"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

func (s *agentService) createEinoReadOnlyEngine(ctx context.Context, d einodef.Definition, c *types.AgentConfig, m chat.Chat, bus *event.EventBus) (interfaces.AgentEngine, error) {
	o, err := s.buildEinoReadOnlyOptions(ctx, d, c, m, bus)
	if err != nil {
		return nil, err
	}
	return einoruntime.New(ctx, o)
}
func (s *agentService) buildEinoReadOnlyOptions(ctx context.Context, d einodef.Definition, c *types.AgentConfig, m chat.Chat, bus *event.EventBus) (einoruntime.Options, error) {
	if bus == nil {
		bus = event.NewEventBus()
	}
	if err := validateEinoDefinitionConfig(d, c); err != nil {
		return einoruntime.Options{}, err
	}
	if s.mcpServiceService == nil || s.mcpManager == nil || s.toolApprovalGate == nil {
		return einoruntime.Options{}, fmt.Errorf("平台MCP授权能力尚未就绪")
	}
	metadata, ok := s.mcpServiceService.(interfaces.MCPMetadataService)
	if !ok {
		return einoruntime.Options{}, fmt.Errorf("MCP工具目录尚未就绪")
	}
	caller := types.CallerFromContext(ctx)
	principal, _ := types.PrincipalFromContext(ctx)
	serviceID := c.MCPServices[0]
	checkIdentity := func(current context.Context) error {
		p, _ := types.PrincipalFromContext(current)
		if caller.TenantID == 0 || caller.UserID == "" || types.CallerFromContext(current) != caller || principal.Type != types.PrincipalWebUser || principal.ID != caller.UserID || p != principal || types.MCPOAuthPrincipalFromContext(current) != principal {
			return fmt.Errorf("只读业务查询只支持当前登录用户的本人OAuth身份")
		}
		if tenant, _ := types.TenantIDFromContext(current); tenant != caller.TenantID {
			return fmt.Errorf("只读业务查询不允许切换执行租户")
		}
		return current.Err()
	}
	if err := checkIdentity(ctx); err != nil {
		return einoruntime.Options{}, err
	}
	call := func(current context.Context, in einoruntime.Invocation) (map[string]any, error) {
		if err := checkIdentity(current); err != nil {
			return nil, err
		}
		if !d.AllowsTool(in.Tool) {
			return nil, fmt.Errorf("工具不在已发布只读能力中")
		}
		svc, err := s.mcpServiceService.GetMCPServiceByID(current, caller.TenantID, serviceID)
		if err != nil || svc == nil || !svc.Enabled || svc.TenantID != caller.TenantID || !svc.AuthConfig.IsOAuth() {
			return nil, fmt.Errorf("本人OAuth服务不可用，不能使用共用账号替代")
		}
		snapshot, err := metadata.GetMCPMetadata(current, caller.TenantID, serviceID)
		if err != nil || snapshot == nil || snapshot.Stale {
			return nil, fmt.Errorf("请在MCP设置刷新工具目录后再使用")
		}
		var schema *types.MCPTool
		for _, item := range snapshot.Tools {
			if item != nil && item.Name == in.Tool {
				schema = item
				break
			}
		}
		if schema == nil {
			return nil, fmt.Errorf("已发布工具尚未就绪，请刷新MCP工具目录")
		}
		tool := tools.NewMCPTool(svc, schema, s.mcpManager, s.toolApprovalGate, 600)
		tool.SetCallAuthorizer(func(next context.Context) error {
			if err := checkIdentity(next); err != nil {
				return err
			}
			fresh, e := s.mcpServiceService.GetMCPServiceByID(next, caller.TenantID, serviceID)
			if e != nil || fresh == nil || !fresh.Enabled || !fresh.AuthConfig.IsOAuth() || fresh.TransportType != svc.TransportType || !sameServiceURL(fresh.URL, svc.URL) {
				return fmt.Errorf("MCP服务授权配置已改变，请重新发起")
			}
			return nil
		})
		args, err := json.Marshal(in.Args)
		if err != nil {
			return nil, err
		}
		if err = tool.ValidateArguments(args); err != nil {
			return nil, fmt.Errorf("只读业务参数不符合工具定义")
		}
		tool.PinArguments(args)
		requestID, _ := types.RequestIDFromContext(current)
		operation, done := context.WithTimeout(current, 60*time.Second)
		defer done()
		operation = tools.WithToolExecContext(operation, &tools.ToolExecContext{SessionID: in.SessionID, AssistantMessageID: in.MessageID, RequestID: requestID, ToolCallID: in.ToolCallID, UserID: principal.StorageID(), EventBus: bus, ApprovalCtx: current, ExecTimeout: 60 * time.Second})
		result, err := tool.Execute(operation, args)
		if err != nil || result == nil || !result.Success {
			return nil, fmt.Errorf("只读业务查询未完成（权限、授权或服务异常），未发布客户数据")
		}
		return decodeEinoBusinessResult(result)
	}
	revalidate := func(current context.Context) (types.SearchTargets, error) { return nil, checkIdentity(current) }
	o := einoruntime.Options{Definition: d.ID, ReadOnlyServiceID: serviceID, Config: c, Model: m, Events: bus, Revalidate: revalidate, Retrieve: func(context.Context, types.SearchTargets, []string) ([]*types.SearchResult, error) {
		return nil, fmt.Errorf("本定义不提供知识检索")
	}, FilterEvidence: func(current context.Context, _ types.SearchTargets, refs []*types.SearchResult) ([]*types.SearchResult, error) {
		if len(refs) > 0 {
			return nil, fmt.Errorf("业务工具结果不是知识引用")
		}
		return nil, checkIdentity(current)
	}, CallReadOnly: call}
	if d.Knowledge {
		if c.EinoRevalidate == nil || c.EinoFilterEvidence == nil {
			return einoruntime.Options{}, fmt.Errorf("知识授权能力尚未就绪")
		}
		o.Revalidate = func(current context.Context) (types.SearchTargets, error) {
			if err := checkIdentity(current); err != nil {
				return nil, err
			}
			return c.EinoRevalidate(current)
		}
		o.FilterEvidence = func(current context.Context, scope types.SearchTargets, refs []*types.SearchResult) ([]*types.SearchResult, error) {
			if err := checkIdentity(current); err != nil {
				return nil, err
			}
			return c.EinoFilterEvidence(current, scope, refs)
		}
	}
	return o, nil
}

func decodeEinoBusinessResult(result *types.ToolResult) (map[string]any, error) {
	raw, err := json.Marshal(result.Data["content_items"])
	if err != nil {
		return nil, err
	}
	var items []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &items) != nil || len(items) != 1 || items[0].Type != "text" || len(items[0].Text) > 128*1024 {
		return nil, fmt.Errorf("只读业务结果结构不受支持")
	}
	decoder := json.NewDecoder(strings.NewReader(items[0].Text))
	decoder.UseNumber()
	var data map[string]any
	if err = decoder.Decode(&data); err != nil || data == nil {
		return nil, fmt.Errorf("只读业务结果不是JSON对象")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("只读业务结果存在额外内容")
	}
	return data, nil
}

func sameServiceURL(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
