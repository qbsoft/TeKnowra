package service

import (
	"context"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowcatalog"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"slices"
	"time"
)

type LangflowCatalogService struct {
	cfg      *config.Config
	agents   interfaces.CustomAgentService
	models   interfaces.ModelService
	kbs      interfaces.KnowledgeBaseService
	mcps     interfaces.MCPServiceService
	users    interfaces.UserService
	members  interfaces.TenantMemberService
	sessions interfaces.SessionService
	tickets  *langflowcatalog.Sessions
}

func NewLangflowCatalogService(cfg *config.Config, agents interfaces.CustomAgentService, models interfaces.ModelService, kbs interfaces.KnowledgeBaseService, mcps interfaces.MCPServiceService, users interfaces.UserService, members interfaces.TenantMemberService, sessions interfaces.SessionService) *LangflowCatalogService {
	s := &LangflowCatalogService{cfg: cfg, agents: agents, models: models, kbs: kbs, mcps: mcps, users: users, members: members, sessions: sessions}
	s.tickets = langflowcatalog.NewSessions(s.Catalog)
	return s
}
func (s *LangflowCatalogService) Create(ctx context.Context, id string) (string, time.Time, error) {
	return s.tickets.Create(ctx, id)
}
func (s *LangflowCatalogService) Read(ctx context.Context, ticket string) (langflowcatalog.Catalog, error) {
	return s.tickets.Read(ctx, ticket)
}
func (s *LangflowCatalogService) Catalog(ctx context.Context, id string) (langflowcatalog.Catalog, error) {
	fail := func() (langflowcatalog.Catalog, error) {
		return langflowcatalog.Catalog{}, fmt.Errorf("无权编辑该Langflow绑定或连接已失效")
	}
	b, ok := s.cfg.DraftLangflowBinding(id)
	if !ok {
		return fail()
	}
	return s.catalogWithBinding(ctx, id, b)
}
func (s *LangflowCatalogService) catalogWithBinding(ctx context.Context, id string, b langflowdef.Binding) (langflowcatalog.Catalog, error) {
	fail := func() (langflowcatalog.Catalog, error) {
		return langflowcatalog.Catalog{}, fmt.Errorf("无权编辑该Langflow绑定或连接已失效")
	}
	caller := types.CallerFromContext(ctx)
	p, _ := types.PrincipalFromContext(ctx)
	if caller.TenantID != b.WorkspaceID || caller.UserID == "" || p.Type != types.PrincipalWebUser || p.ID != caller.UserID {
		return fail()
	}
	user, err := s.users.GetUserByID(ctx, caller.UserID)
	if err != nil || user == nil || !user.IsActive {
		return fail()
	}
	member, err := s.members.GetMembership(ctx, caller.UserID, caller.TenantID)
	if err != nil || member == nil || member.Status != types.TenantMemberStatusActive {
		return fail()
	}
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, member.Role)
	// Clear cached grants; every refresh evaluates current directory permissions.
	ctx = context.WithValue(ctx, types.KBGrantsContextKey, struct{}{})
	agent, err := s.agents.GetAgentByID(ctx, id)
	if err != nil || agent == nil || agent.TenantID != b.WorkspaceID || (!member.Role.HasPermission(types.TenantRoleAdmin) && (agent.CreatedBy != caller.UserID || !member.Role.HasPermission(types.TenantRoleContributor))) {
		return fail()
	}
	result := langflowcatalog.Catalog{AgentID: id, FlowID: b.FlowID, Models: []langflowcatalog.Resource{}, KnowledgeBases: []langflowcatalog.Resource{}, Services: []langflowcatalog.Service{}}
	result.AllowSessionAttachments = agent.Config.AllowSessionAttachments
	if len(b.Skills) > 0 {
		state, err := s.skillState(ctx, b)
		if err != nil {
			return result, err
		}
		for _, skill := range state.Descriptors {
			result.Skills = append(result.Skills, langflowcatalog.SkillResource{ID: skill.ID, Name: skill.Name, Description: skill.Description})
		}
	}
	for _, id := range b.Models {
		m, err := s.models.GetModelByID(ctx, id)
		if err != nil || m == nil || (!m.IsBuiltin && m.TenantID != b.WorkspaceID) || m.Type != types.ModelTypeKnowledgeQA || m.Status != types.ModelStatusActive {
			continue
		}
		name := m.DisplayName
		if name == "" {
			name = m.Name
		}
		result.Models = append(result.Models, langflowcatalog.Resource{ID: id, Name: name})
	}
	kbIDs := []string{}
	for _, id := range b.KnowledgeBases {
		if len(agent.Config.KnowledgeBases) == 0 || slices.Contains(agent.Config.KnowledgeBases, id) {
			kbIDs = append(kbIDs, id)
		}
	}
	if len(kbIDs) > 0 {
		ss, ok := s.sessions.(*sessionService)
		if !ok {
			return fail()
		}
		targets, err := ss.buildSearchTargets(ctx, b.WorkspaceID, kbIDs, nil, nil)
		if err != nil {
			return fail()
		}
		seen := map[string]bool{}
		for _, target := range targets {
			if target == nil || seen[target.KnowledgeBaseID] {
				continue
			}
			kb, err := s.kbs.GetKnowledgeBaseByIDOnly(ctx, target.KnowledgeBaseID)
			if err != nil || kb == nil {
				continue
			}
			seen[kb.ID] = true
			result.KnowledgeBases = append(result.KnowledgeBases, langflowcatalog.Resource{ID: kb.ID, Name: kb.Name})
		}
	}
	metadata, ok := s.mcps.(interfaces.MCPMetadataService)
	if len(b.Tools) > 0 && !ok {
		return fail()
	}
	seen := map[string]int{}
	for _, t := range b.Tools {
		if agent.Config.MCPSelectionMode != "selected" || !slices.Contains(agent.Config.MCPServices, t.ServiceID) || !slices.Contains(agent.Config.AllowedTools, tools.AgentMCPToolKey(t.ServiceID, t.Name)) {
			continue
		}
		svc, err := s.mcps.GetMCPServiceByID(ctx, b.WorkspaceID, t.ServiceID)
		if err != nil || svc == nil || !svc.Enabled || (!svc.IsBuiltin && svc.TenantID != b.WorkspaceID) {
			continue
		}
		snapshot, err := metadata.GetMCPMetadata(ctx, b.WorkspaceID, t.ServiceID)
		if err != nil || snapshot == nil || snapshot.Stale {
			continue
		}
		for _, tool := range snapshot.Tools {
			if tool == nil || tool.Name != t.Name {
				continue
			}
			idx, exists := seen[t.ServiceID]
			if !exists {
				idx = len(result.Services)
				seen[t.ServiceID] = idx
				result.Services = append(result.Services, langflowcatalog.Service{ID: t.ServiceID, Name: svc.Name, Tools: []langflowcatalog.Tool{}})
			}
			result.Services[idx].Tools = append(result.Services[idx].Tools, langflowcatalog.Tool{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
		}
	}
	return result, nil
}

func (s *LangflowCatalogService) skillState(ctx context.Context, b langflowdef.Binding) (langflowSkillState, error) {
	if len(b.Skills) == 0 {
		return langflowSkillState{}, nil
	}
	ss, ok := s.sessions.(*sessionService)
	if !ok {
		return langflowSkillState{}, fmt.Errorf("技能宿主服务不可用")
	}
	native, ok := ss.agentService.(*agentService)
	if !ok {
		return langflowSkillState{}, fmt.Errorf("技能宿主执行器不可用")
	}
	return native.langflowSkills(ctx, b)
}
