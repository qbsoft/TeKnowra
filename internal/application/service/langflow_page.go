package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowapi"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"slices"
	"time"
)

type LangflowDraftRequest struct {
	Mode              string                         `json:"mode"`
	FlowID            string                         `json:"flow_id,omitempty"`
	StructuredScripts []langflowdef.StructuredScript `json:"structured_scripts,omitempty"`
}
type LangflowDraftView struct {
	AgentID string `json:"agent_id"`
	FlowID  string `json:"flow_id"`
	Name    string `json:"name"`
}

func (s *LangflowReleaseService) EditableAgent(ctx context.Context, id string) (*types.CustomAgent, error) {
	return s.catalog.editableAgent(ctx, id)
}
func (s *LangflowReleaseService) DraftScripts(id string) []langflowdef.StructuredScript {
	b, _ := s.cfg.DraftLangflowBinding(id)
	return b.StructuredScripts
}
func (s *LangflowReleaseService) DraftMetadata(id string) (langflowdef.Binding, bool, bool) {
	b, ok := s.cfg.DraftLangflowBinding(id)
	managed := false
	if store := s.cfg.LangflowReleases(); store != nil {
		_, managed = store.Draft(id)
	}
	return b, ok, managed
}
func (s *LangflowReleaseService) ActiveMetadata(id string) (langflowrelease.Release, bool) {
	if store := s.cfg.LangflowReleases(); store != nil {
		return store.Active(id)
	}
	return langflowrelease.Release{}, false
}

func (s *LangflowCatalogService) editableAgent(ctx context.Context, id string) (*types.CustomAgent, error) {
	fail := func() (*types.CustomAgent, error) {
		return nil, fmt.Errorf("无权配置该智能体的Langflow接入")
	}
	caller := types.CallerFromContext(ctx)
	p, _ := types.PrincipalFromContext(ctx)
	if caller.UserID == "" || caller.TenantID == 0 || p.Type != types.PrincipalWebUser || p.ID != caller.UserID {
		return fail()
	}
	user, err := s.users.GetUserByID(ctx, caller.UserID)
	if err != nil || user == nil || user.ID != caller.UserID || !user.IsActive {
		return fail()
	}
	member, err := s.members.GetMembership(ctx, caller.UserID, caller.TenantID)
	if err != nil || member == nil || member.UserID != caller.UserID || member.TenantID != caller.TenantID || member.Status != types.TenantMemberStatusActive || !member.Role.HasPermission(types.TenantRoleContributor) {
		return fail()
	}
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, member.Role)
	a, err := s.agents.GetAgentByID(ctx, id)
	if err != nil || a == nil || a.ID != id || a.IsBuiltin || a.TenantID != caller.TenantID || (!member.Role.HasPermission(types.TenantRoleAdmin) && a.CreatedBy != caller.UserID) {
		return fail()
	}
	return a, nil
}

// Resource authority comes from the saved host Agent, never submitted graph claims.
func pageLangflowBinding(a *types.CustomAgent, flow string, graph json.RawMessage, policies []langflowdef.StructuredScript) (langflowdef.Binding, error) {
	b := langflowdef.Binding{AgentID: a.ID, WorkspaceID: a.TenantID, FlowID: flow, Models: []string{a.Config.ModelID}, AllowSessionAttachments: a.Config.AllowSessionAttachments, StructuredScripts: policies, AllowSkillScripts: len(policies) > 0}
	c := a.Config
	if c.ExecutionEngine != "langflow" {
		return b, fmt.Errorf("请先保存Langflow执行引擎选择")
	}
	if err := c.ValidateExecutionEngine(); err != nil {
		return b, err
	}
	if c.MemoryEnabled != nil && *c.MemoryEnabled || c.WebSearchEnabled {
		return b, fmt.Errorf("Langflow接入请关闭长期记忆、网络搜索和本机浏览器")
	}
	switch c.KBSelectionMode {
	case "none":
	case "selected":
		b.KnowledgeBases = slices.Clone(c.KnowledgeBases)
	default:
		return b, fmt.Errorf("请明确选择指定知识库或不使用知识库")
	}
	if c.MCPSelectionMode == "selected" {
		for _, key := range c.AllowedTools {
			svc, name, ok := tools.ParseAgentMCPToolKey(key)
			if ok && slices.Contains(c.MCPServices, svc) {
				b.Tools = append(b.Tools, langflowdef.ToolBinding{ServiceID: svc, Name: name})
			}
		}
		for i := range b.Tools {
			if b.Tools[i].Name == "get_customer_credit_control" {
				if !b.AllowsTool(b.Tools[i].ServiceID, "check_customer_access") {
					return b, fmt.Errorf("CRM信用查询须同时选择check_customer_access访问复核工具")
				}
				b.Tools[i].Guard = "crm-credit"
			}
		}
	} else if c.MCPSelectionMode != "none" && c.MCPSelectionMode != "" {
		return b, fmt.Errorf("请明确选择MCP服务与工具")
	}
	switch c.SkillsSelectionMode {
	case "none", "":
	case "selected":
		b.Skills = slices.Clone(c.SelectedSkills)
		if len(b.Skills) > 0 {
			b.SandboxConfigID = c.SandboxConfigID
		}
	default:
		return b, fmt.Errorf("请明确指定技能")
	}
	var root struct {
		Nodes []struct {
			ID   string `json:"id"`
			Data struct {
				Type string `json:"type"`
			} `json:"data"`
		} `json:"nodes"`
	}
	if json.Unmarshal(graph, &root) != nil {
		return b, fmt.Errorf("流程数据无效")
	}
	for _, n := range root.Nodes {
		switch langflowdef.ComponentKind(n.Data.Type) {
		case "TeKnowraModel":
			b.Nodes = append(b.Nodes, langflowdef.NodeBinding{ID: n.ID, Kind: "model"})
		case "TeKnowraKnowledge":
			b.Nodes = append(b.Nodes, langflowdef.NodeBinding{ID: n.ID, Kind: "knowledge"})
		case "TeKnowraMCP":
			b.Nodes = append(b.Nodes, langflowdef.NodeBinding{ID: n.ID, Kind: "mcp"})
		case "TeKnowraSkills":
			b.Nodes = append(b.Nodes, langflowdef.NodeBinding{ID: n.ID, Kind: "skill"})
		case "ReviewedExpert", "ExpertResults":
			b.ReviewedExperts = true
		case "Agent", "ChatInput", "ChatOutput":
		default:
			return b, fmt.Errorf("流程包含当前未批准的组件")
		}
	}
	b.ReviewedExpertPolicy = langflowrelease.GraphReviewedExpertPolicy(graph)
	return b, b.Validate()
}

func (s *LangflowReleaseService) DraftOptions(ctx context.Context, id string) ([]LangflowDraftView, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("Langflow固定发布服务未启用")
	}
	a, err := s.catalog.editableAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	out := []LangflowDraftView{}
	seen := map[string]bool{}
	for _, b := range s.cfg.LangflowDrafts() {
		if b.WorkspaceID != a.TenantID || seen[b.FlowID] {
			continue
		}
		source, e := s.catalog.editableAgent(ctx, b.AgentID)
		if e != nil {
			continue
		}
		out = append(out, LangflowDraftView{AgentID: b.AgentID, FlowID: b.FlowID, Name: source.Name})
		seen[b.FlowID] = true
	}
	return out, nil
}

func (s *LangflowReleaseService) SetupDraft(ctx context.Context, id string, in LangflowDraftRequest) (langflowdef.Binding, error) {
	if !s.Enabled() {
		return langflowdef.Binding{}, fmt.Errorf("Langflow固定发布服务未启用")
	}
	s.manager.LockDraftSetup()
	defer s.manager.UnlockDraftSetup()
	a, err := s.catalog.editableAgent(ctx, id)
	if err != nil {
		return langflowdef.Binding{}, err
	}
	if prior, ok := s.cfg.DraftLangflowBinding(id); ok && prior.WorkspaceID != a.TenantID {
		return langflowdef.Binding{}, fmt.Errorf("该标识已有其他空间绑定")
	}
	if _, eino := s.cfg.EinoDefinition(id); eino {
		return langflowdef.Binding{}, fmt.Errorf("该智能体已绑定Eino，不能同时接入Langflow")
	}
	if a.Config.ExecutionEngine != "langflow" {
		return langflowdef.Binding{}, fmt.Errorf("请保存Langflow执行引擎选择后再配置")
	}
	call, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var graph json.RawMessage
	flow := in.FlowID
	switch in.Mode {
	case "create":
		seed := langflowdef.Binding{AgentID: a.ID, WorkspaceID: a.TenantID, Models: []string{a.Config.ModelID}, KnowledgeBases: slices.Clone(a.Config.KnowledgeBases), Skills: slices.Clone(a.Config.SelectedSkills), SandboxConfigID: a.Config.SandboxConfigID}
		if a.Config.KBSelectionMode == "none" {
			seed.KnowledgeBases = nil
		}
		if a.Config.SkillsSelectionMode == "none" {
			seed.Skills = nil
			seed.SandboxConfigID = ""
		}
		for _, key := range a.Config.AllowedTools {
			svc, name, ok := tools.ParseAgentMCPToolKey(key)
			if ok && a.Config.MCPSelectionMode == "selected" && slices.Contains(a.Config.MCPServices, svc) {
				seed.Tools = append(seed.Tools, langflowdef.ToolBinding{ServiceID: svc, Name: name})
			}
		}
		catalog, e := s.catalog.catalogWithBinding(call, id, seed)
		if e != nil {
			return seed, e
		}
		starterConfig := a.Config
		starterConfig.SystemPrompt, _ = s.cfg.ResolveCustomAgentPrompts(a)
		graph, err = pageStarterGraph(catalog, starterConfig, in.StructuredScripts)
		flow = uuid.NewString()
	case "associate":
		options, e := s.DraftOptions(call, id)
		if e != nil {
			return langflowdef.Binding{}, e
		}
		allowed := false
		for _, o := range options {
			if o.FlowID == flow {
				allowed = true
				break
			}
		}
		if !allowed {
			return langflowdef.Binding{}, fmt.Errorf("只能关联当前空间有权配置的已登记编辑流程")
		}
		src, e := s.manager.Editor.GetFlow(call, flow)
		if e != nil {
			return langflowdef.Binding{}, e
		}
		if src.UserID == s.manager.RuntimeOwnerID {
			return langflowdef.Binding{}, fmt.Errorf("不能把冻结运行版本作为编辑草稿")
		}
		graph = src.Data
	case "sync":
		prior, ok := s.cfg.DraftLangflowBinding(id)
		if !ok || prior.WorkspaceID != a.TenantID {
			return prior, fmt.Errorf("请先创建或关联流程")
		}
		flow = prior.FlowID
		src, e := s.manager.Editor.GetFlow(call, flow)
		if e != nil {
			return prior, e
		}
		if src.UserID == s.manager.RuntimeOwnerID {
			return prior, fmt.Errorf("不能修改冻结运行版本")
		}
		graph = src.Data
	default:
		return langflowdef.Binding{}, fmt.Errorf("无效的草稿操作")
	}
	if err != nil {
		return langflowdef.Binding{}, err
	}
	b, err := pageLangflowBinding(a, flow, graph, in.StructuredScripts)
	if err != nil {
		return b, err
	}
	catalog, err := s.catalog.catalogWithBinding(call, id, b)
	if err != nil {
		return b, err
	}
	if err = completeReleaseCatalog(catalog, b); err != nil {
		return b, err
	}
	components, err := releaseBindingComponents(call, s.runtime, b)
	if err != nil {
		return b, err
	}
	if err = langflowrelease.ValidateGraph(graph, b, components.Approved); err != nil {
		return b, err
	}
	if in.Mode != "sync" {
		creator, ok := s.manager.Editor.(interface {
			CreateDraftFlow(context.Context, string, json.RawMessage) (langflowapi.Flow, error)
		})
		if !ok {
			return b, fmt.Errorf("编辑流程创建服务不可用")
		}
		created, e := creator.CreateDraftFlow(call, "TeKnowra draft "+a.Name, graph)
		if e != nil {
			return b, e
		}
		if created.UserID == s.manager.RuntimeOwnerID {
			return b, fmt.Errorf("编辑与运行身份未隔离")
		}
		if e = langflowrelease.ValidateGraph(created.Data, b, components.Approved); e != nil {
			return b, e
		}
		b.FlowID = created.ID
	}
	// Recheck after remote operations. An unbound private orphan is retained on
	// revocation instead of granting a host binding or deleting any old data.
	current, err := s.catalog.editableAgent(ctx, id)
	if err != nil {
		return b, err
	}
	currentRaw, _ := json.Marshal(current.Config)
	originalRaw, _ := json.Marshal(a.Config)
	if string(currentRaw) != string(originalRaw) {
		return b, fmt.Errorf("配置已变化，请保存后重新配置流程")
	}
	latest, latestErr := s.catalog.catalogWithBinding(ctx, id, b)
	if latestErr != nil {
		return b, latestErr
	}
	if err = completeReleaseCatalog(latest, b); err != nil {
		return b, err
	}
	return b, s.manager.Store.PutDraft(b)
}
