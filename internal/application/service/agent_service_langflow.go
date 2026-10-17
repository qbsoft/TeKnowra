package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/langflowruntime"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/models/rerank"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

func validateLangflowCapabilities(b langflowdef.Binding, c *types.AgentConfig) error {
	if c != nil && c.ExecutionEngine != "" && c.ExecutionEngine != "langflow" {
		return fmt.Errorf("该智能体仍有Langflow绑定，请保持Langflow执行引擎")
	}
	if c == nil || c.WebSearchEnabled || c.LocalBrowserEnabled || c.MemoryEnabled == nil || *c.MemoryEnabled {
		return fmt.Errorf("Langflow暂不支持浏览器或长期记忆")
	}
	if b.AllowSessionAttachments != c.AllowSessionAttachments {
		return fmt.Errorf("会话附件策略已变化，请重新发布")
	}
	if err := b.ValidateScriptPolicies(); err != nil {
		return err
	}
	if len(b.Skills) == 0 {
		if c.SkillsEnabled || c.SandboxConfigID != "" {
			return fmt.Errorf("请先发布技能与执行环境绑定")
		}
	} else {
		if !c.SkillsEnabled || c.SandboxConfigID != b.SandboxConfigID {
			return fmt.Errorf("技能执行环境或权限不匹配")
		}
		for _, name := range b.Skills {
			if len(c.AllowedSkills) > 0 && !slices.Contains(c.AllowedSkills, name) {
				return fmt.Errorf("技能不在Agent选择范围")
			}
		}
	}
	for _, id := range b.KnowledgeBases {
		found := false
		for _, target := range c.SearchTargets {
			if target != nil && target.KnowledgeBaseID == id {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("绑定知识库不在当前授权范围")
		}
	}
	for _, t := range b.Tools {
		if c.MCPSelectionMode != "selected" || !slices.Contains(c.MCPServices, t.ServiceID) || !slices.Contains(c.AllowedTools, tools.AgentMCPToolKey(t.ServiceID, t.Name)) {
			return fmt.Errorf("绑定工具不在Agent授权范围")
		}
		if t.Guard == "crm-credit" && (!b.AllowsTool(t.ServiceID, "check_customer_access") || t.Name != "get_customer_credit_control") {
			return fmt.Errorf("CRM信用策略缺少访问校验工具")
		}
	}
	return nil
}

func (s *agentService) createLangflowEngine(ctx context.Context, b langflowdef.Binding, c *types.AgentConfig, primary chat.Chat, rank rerank.Reranker, bus *event.EventBus, sessionID, assistantMessageID string) (interfaces.AgentEngine, error) {
	if err := validateLangflowCapabilities(b, c); err != nil {
		return nil, err
	}
	caller := types.CallerFromContext(ctx)
	principal, _ := types.PrincipalFromContext(ctx)
	var initialRole types.TenantRole
	var initialSystemAdmin bool
	initialized := false
	skillBaseline := ""
	check := func(current context.Context) error {
		p, _ := types.PrincipalFromContext(current)
		tenant, _ := types.TenantIDFromContext(current)
		if caller.UserID == "" || caller.TenantID == 0 || (caller.TenantID != b.WorkspaceID && !c.SharedAgentReadOnly) || tenant != b.WorkspaceID || types.CallerFromContext(current) != caller || principal.Type != types.PrincipalWebUser || p != principal || types.MCPOAuthPrincipalFromContext(current) != principal {
			return fmt.Errorf("Langflow只允许当前登录员工在原工作空间执行")
		}
		if err := current.Err(); err != nil {
			return err
		}
		if s.db == nil {
			return fmt.Errorf("身份授权存储未就绪")
		}
		var user types.User
		var member types.TenantMember
		if err := s.db.WithContext(current).Where("id = ?", caller.UserID).First(&user).Error; err != nil {
			return fmt.Errorf("员工身份已失效")
		}
		if err := s.db.WithContext(current).Where("user_id = ? AND tenant_id = ?", caller.UserID, caller.TenantID).First(&member).Error; err != nil {
			return fmt.Errorf("员工工作空间授权已失效")
		}
		agent, err := s.loadLangflowRuntimeAgent(current, b, caller, c.SharedAgentReadOnly)
		if err != nil {
			return err
		}
		if err := validateLangflowLiveAuthority(b, caller, &user, &member, agent, c.SharedAgentReadOnly); err != nil {
			return err
		}
		if !initialized {
			initialRole = member.Role
			initialSystemAdmin = user.IsSystemAdmin
			initialized = true
		} else if member.Role != initialRole || user.IsSystemAdmin != initialSystemAdmin {
			return fmt.Errorf("员工角色已变化，请重新发起任务")
		}
		if len(b.Skills) > 0 {
			live, err := s.langflowSkills(current, b)
			if err != nil {
				return err
			}
			if skillBaseline == "" {
				skillBaseline = live.Digest
			} else if live.Digest != skillBaseline {
				return fmt.Errorf("技能版本或执行镜像已变化，请重新发布")
			}
		}
		return nil
	}
	if err := check(ctx); err != nil {
		return nil, err
	}
	o := hostcapability.Options{Config: c, Events: bus, FlowURL: os.Getenv("TEKNOWRA_LANGFLOW_URL"), LocalAutoLogin: os.Getenv("TEKNOWRA_LANGFLOW_LOCAL_TEST") == "true"}
	if len(b.Skills) > 0 {
		var err error
		o.Skills, o.CallSkill, o.Cleanup, err = s.langflowSkillCaller(ctx, b, c, sessionID, check)
		if err != nil {
			return nil, err
		}
	}
	o.Revalidate = func(current context.Context) (types.SearchTargets, error) {
		if err := check(current); err != nil {
			return nil, err
		}
		if len(b.KnowledgeBases) > 0 {
			if c.RuntimeRevalidate == nil {
				return nil, fmt.Errorf("知识授权回调未准备")
			}
			return c.RuntimeRevalidate(current)
		}
		return nil, nil
	}
	o.FilterEvidence = c.RuntimeFilterEvidence
	o.ResolveModel = func(current context.Context, id string) (chat.Chat, error) {
		if err := check(current); err != nil {
			return nil, err
		}
		if !slices.Contains(b.Models, id) {
			return nil, fmt.Errorf("模型未绑定")
		}
		model, err := s.modelService.GetModelByID(current, id)
		if err != nil || model == nil || (!model.IsBuiltin && model.TenantID != b.WorkspaceID) || model.Type != types.ModelTypeKnowledgeQA || model.Status != types.ModelStatusActive {
			return nil, fmt.Errorf("模型不可用或无权使用")
		}
		return s.modelService.GetChatModel(current, id)
	}
	o.Retrieve = func(current context.Context, scope types.SearchTargets, queries []string) ([]*types.SearchResult, error) {
		if err := check(current); err != nil {
			return nil, err
		}
		local := *c
		local.SearchTargets = cloneEinoTargets(scope)
		local.MCPSelectionMode = "none"
		local.MCPServices = nil
		local.AllowedTools = []string{tools.ToolSearchKnowledge}
		registry := tools.NewToolRegistry()
		defer registry.Cleanup(current)
		if err := s.registerTools(current, registry, &local, rank, primary, sessionID); err != nil {
			return nil, err
		}
		args, _ := json.Marshal(map[string]any{"query": strings.Join(queries, "；")})
		result, err := registry.ExecuteTool(current, tools.ToolSearchKnowledge, args)
		if err != nil || result == nil || !result.Success {
			return nil, fmt.Errorf("知识检索未完成")
		}
		raw, err := json.Marshal(result.Data["results"])
		if err != nil {
			return nil, err
		}
		var rows []map[string]any
		if err = json.Unmarshal(raw, &rows); err != nil {
			return nil, err
		}
		for _, r := range rows {
			if r["chunk_id"] != nil {
				r["id"] = r["chunk_id"]
			} else {
				r["id"] = r["faq_id"]
			}
		}
		raw, _ = json.Marshal(rows)
		var refs []*types.SearchResult
		err = json.Unmarshal(raw, &refs)
		return refs, err
	}
	if len(b.Tools) > 0 {
		metadata, ok := s.mcpServiceService.(interfaces.MCPMetadataService)
		if !ok || s.mcpManager == nil || s.toolApprovalGate == nil {
			return nil, fmt.Errorf("MCP能力未准备")
		}
		if err := s.prepareLangflowSharedMCP(ctx, b, c, metadata, bus, sessionID, assistantMessageID, check); err != nil {
			return nil, err
		}
		schemas := map[string]*types.MCPTool{}
		for _, t := range b.Tools {
			snapshot, err := metadata.GetMCPMetadata(ctx, b.WorkspaceID, t.ServiceID)
			if err != nil || snapshot == nil || snapshot.Stale {
				return nil, fmt.Errorf("请先同步MCP工具目录")
			}
			for _, schema := range snapshot.Tools {
				if schema != nil && schema.Name == t.Name {
					schemas[t.ServiceID+"/"+t.Name] = schema
					o.Tools = append(o.Tools, hostcapability.ToolDescriptor{ServiceID: t.ServiceID, Name: t.Name, Description: schema.Description, InputSchema: schema.InputSchema})
					break
				}
			}
			if schemas[t.ServiceID+"/"+t.Name] == nil {
				return nil, fmt.Errorf("绑定工具在目录中不存在")
			}
		}
		rawCall := func(current context.Context, in hostcapability.Invocation) (*types.ToolResult, error) {
			if err := check(current); err != nil {
				return nil, err
			}
			if !b.AllowsTool(in.ServiceID, in.Tool) {
				return nil, fmt.Errorf("工具未绑定")
			}
			svc, err := s.mcpServiceService.GetMCPServiceByID(current, b.WorkspaceID, in.ServiceID)
			if err != nil || svc == nil || !svc.Enabled || (!svc.IsBuiltin && svc.TenantID != b.WorkspaceID) {
				return nil, fmt.Errorf("MCP服务不可用")
			}
			fresh, err := metadata.GetMCPMetadata(current, b.WorkspaceID, in.ServiceID)
			if err != nil || fresh == nil || fresh.Stale {
				return nil, fmt.Errorf("MCP目录已失效")
			}
			initial := schemas[in.ServiceID+"/"+in.Tool]
			var schema *types.MCPTool
			for _, item := range fresh.Tools {
				if item != nil && item.Name == in.Tool {
					schema = item
					break
				}
			}
			if schema == nil || string(schema.InputSchema) != string(initial.InputSchema) {
				return nil, fmt.Errorf("MCP工具定义已变化，请重新配置")
			}
			tool := tools.NewMCPTool(svc, schema, s.mcpManager, s.toolApprovalGate, 600)
			tool.SetCallAuthorizer(func(next context.Context) error {
				if err := check(next); err != nil {
					return err
				}
				service, err := s.mcpServiceService.GetMCPServiceByID(next, b.WorkspaceID, in.ServiceID)
				if err != nil || service == nil || !service.Enabled || service.TransportType != svc.TransportType || !sameServiceURL(service.URL, svc.URL) || service.AuthConfig.IsOAuth() != svc.AuthConfig.IsOAuth() {
					return fmt.Errorf("MCP服务授权配置已改变")
				}
				return nil
			})
			args, err := json.Marshal(in.Args)
			if err != nil {
				return nil, err
			}
			if err = tool.ValidateArguments(args); err != nil {
				return nil, fmt.Errorf("工具参数不符合schema")
			}
			tool.PinArguments(args)
			operation, cancel := context.WithTimeout(current, 60*time.Second)
			defer cancel()
			requestID, _ := types.RequestIDFromContext(current)
			operation = tools.WithToolExecContext(operation, &tools.ToolExecContext{SessionID: in.SessionID, AssistantMessageID: in.MessageID, RequestID: requestID, ToolCallID: in.ToolCallID, UserID: principal.StorageID(), EventBus: bus, ApprovalCtx: current, ExecTimeout: 60 * time.Second})
			result, err := tool.Execute(operation, args)
			if err != nil || result == nil || !result.Success {
				return nil, fmt.Errorf("受控工具执行失败，未发布结果")
			}
			return result, nil
		}
		var mu sync.Mutex
		customers := map[string]hostcapability.Invocation{}
		access := func(current context.Context, in hostcapability.Invocation, id string) error {
			in.Tool = "check_customer_access"
			in.Args = map[string]any{"customer_id": id}
			result, err := rawCall(current, in)
			if err != nil {
				return err
			}
			data, err := decodeLangflowBusinessObject(result)
			if err != nil || data["customer_id"] != id || data["readable"] != true {
				return fmt.Errorf("客户不可访问")
			}
			return nil
		}
		o.CallTool = func(current context.Context, in hostcapability.Invocation) (map[string]any, error) {
			guard := ""
			for _, t := range b.Tools {
				if t.ServiceID == in.ServiceID && t.Name == in.Tool {
					guard = t.Guard
				}
			}
			id := ""
			if guard == "crm-credit" {
				var ok bool
				id, ok = in.Args["customer_id"].(string)
				if !ok || id == "" {
					return nil, fmt.Errorf("客户ID必须为完整字符串")
				}
				if err := access(current, in, id); err != nil {
					return nil, err
				}
			}
			result, err := rawCall(current, in)
			if err != nil {
				return nil, err
			}
			if guard == "crm-credit" {
				data, err := decodeLangflowBusinessObject(result)
				if err != nil || data["customerId"] != id || data["creditAvailable"] == nil {
					return nil, fmt.Errorf("信用结果不完整")
				}
				mu.Lock()
				customers[in.ServiceID+"/"+id] = in
				mu.Unlock()
				return data, nil
			}
			return normalizeLangflowToolResult(result)
		}
		o.BeforePublish = func(current context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			for key, invocation := range customers {
				id := strings.TrimPrefix(key, invocation.ServiceID+"/")
				if err := access(current, invocation, id); err != nil {
					return err
				}
			}
			return nil
		}
	}
	if store := s.cfg.LangflowReleases(); store != nil {
		release, ok := store.Get(b.AgentID, b.ReleaseID)
		if !ok || release.WorkspaceID != b.WorkspaceID || release.Binding.FlowID != b.FlowID {
			return nil, fmt.Errorf("请先发布该智能体的固定版本")
		}
		manager, client, creds, err := releaseManager(s.cfg)
		if err != nil {
			return nil, err
		}
		o.RequireRelease = true
		o.LocalAutoLogin = false
		o.ServiceAPIKey = creds.RuntimeAPIKey
		o.VerifyRelease = func(current context.Context) error {
			if err := check(current); err != nil {
				return err
			}
			call, cancel := context.WithTimeout(current, 20*time.Second)
			defer cancel()
			components, err := releaseBindingComponents(call, client, b)
			if err != nil {
				return err
			}
			digest, err := releaseCapabilityDigest(call, s.mcpServiceService, b, s.langflowSkills)
			if err != nil {
				return err
			}
			return manager.Verify(call, release, components.Digest, digest)
		}
	}
	return langflowruntime.New(b, o)
}
