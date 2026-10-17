package service

import (
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"slices"
)

func validateLangflowLiveAuthority(b langflowdef.Binding, caller types.Caller, user *types.User, member *types.TenantMember, agent *types.CustomAgent, sharedAuthorized ...bool) error {
	fail := func() error {
		return fmt.Errorf("员工、工作空间或Agent资源授权已变化，请重新发起任务")
	}
	shared := len(sharedAuthorized) == 1 && sharedAuthorized[0] && caller.TenantID != 0 && caller.TenantID != b.WorkspaceID
	if user == nil || !user.IsActive || user.ID != caller.UserID || member == nil || member.UserID != caller.UserID || member.TenantID != caller.TenantID || member.Status != types.TenantMemberStatusActive || !member.Role.HasPermission(types.TenantRoleViewer) || agent == nil || agent.ID != b.AgentID || agent.TenantID != b.WorkspaceID || (caller.TenantID != b.WorkspaceID && !shared) {
		return fail()
	}
	if (agent.Config.ExecutionEngine != "" && agent.Config.ExecutionEngine != "langflow") || b.AllowSessionAttachments != agent.Config.AllowSessionAttachments {
		return fail()
	}
	for _, kb := range b.KnowledgeBases {
		if agent.Config.KBSelectionMode == "none" || (agent.Config.KBSelectionMode == "selected" && !slices.Contains(agent.Config.KnowledgeBases, kb)) {
			return fail()
		}
	}
	for _, tool := range b.Tools {
		if agent.Config.MCPSelectionMode != "selected" || !slices.Contains(agent.Config.MCPServices, tool.ServiceID) || !slices.Contains(agent.Config.AllowedTools, tools.AgentMCPToolKey(tool.ServiceID, tool.Name)) {
			return fail()
		}
	}
	if err := validateLangflowSkillGrant(b, agent.Config); err != nil {
		return fail()
	}
	return nil
}
