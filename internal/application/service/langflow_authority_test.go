package service

import (
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestLangflowLiveAuthorityRejectsRevokedMemberUserAndTool(t *testing.T) {
	b := langflowdef.Binding{AgentID: "a", WorkspaceID: 1, Tools: []langflowdef.ToolBinding{{ServiceID: "s", Name: "lookup"}}}
	caller := types.Caller{UserID: "u", TenantID: 1}
	user := &types.User{ID: "u", IsActive: true}
	member := &types.TenantMember{UserID: "u", TenantID: 1, Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusActive}
	agent := &types.CustomAgent{ID: "a", TenantID: 1, Config: types.CustomAgentConfig{MCPSelectionMode: "selected", MCPServices: []string{"s"}, AllowedTools: []string{tools.AgentMCPToolKey("s", "lookup")}}}
	if err := validateLangflowLiveAuthority(b, caller, user, member, agent); err != nil {
		t.Fatal(err)
	}
	user.IsActive = false
	if err := validateLangflowLiveAuthority(b, caller, user, member, agent); err == nil {
		t.Fatal("disabled user retained task")
	}
	user.IsActive = true
	member.Status = types.TenantMemberStatusSuspended
	if err := validateLangflowLiveAuthority(b, caller, user, member, agent); err == nil {
		t.Fatal("revoked member retained task")
	}
	member.Status = types.TenantMemberStatusActive
	agent.Config.AllowedTools = nil
	if err := validateLangflowLiveAuthority(b, caller, user, member, agent); err == nil {
		t.Fatal("removed tool retained task")
	}
}
