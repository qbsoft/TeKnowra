package service

import (
	"context"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/application/access"
	"github.com/Tencent/WeKnora/internal/types"
)

// Keep resource ownership and the authenticated receiver separate. The native
// sharing service re-reads organization membership and the exact source share.
func (s *agentService) loadLangflowRuntimeAgent(ctx context.Context, b langflowdef.Binding, caller types.Caller, shared bool) (*types.CustomAgent, error) {
	fail := func() (*types.CustomAgent, error) { return nil, fmt.Errorf("Agent共享或访问授权已失效") }
	if shared {
		if caller.TenantID == 0 || caller.TenantID == b.WorkspaceID || s.agentShareService == nil {
			return fail()
		}
		agent, err := s.agentShareService.GetSharedAgentForTenant(ctx, caller.TenantID, caller.Role, b.AgentID, b.WorkspaceID)
		if err != nil || agent == nil || agent.ID != b.AgentID || agent.TenantID != b.WorkspaceID {
			return fail()
		}
		return agent, nil
	}
	if caller.TenantID != b.WorkspaceID || s.db == nil {
		return fail()
	}
	var agent types.CustomAgent
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", b.AgentID, b.WorkspaceID).First(&agent).Error; err != nil {
		return fail()
	}
	return &agent, nil
}

func (s *sessionService) refreshLangflowSharedKnowledgeContext(ctx context.Context, cfg *types.AgentConfig) (context.Context, error) {
	native, ok := s.agentService.(*agentService)
	if !ok || s.cfg == nil {
		return nil, fmt.Errorf("共享Agent知识授权服务不可用")
	}
	binding, ok := s.cfg.LangflowBinding(cfg.AgentID)
	if !ok {
		return nil, fmt.Errorf("共享Agent未绑定发布流程")
	}
	agent, err := native.loadLangflowRuntimeAgent(ctx, binding, types.CallerFromContext(ctx), true)
	if err != nil {
		return nil, err
	}
	return access.WithSharedAgent(ctx, agent), nil
}
