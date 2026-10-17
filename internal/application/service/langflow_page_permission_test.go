package service

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"testing"
)

type pageUsers struct {
	interfaces.UserService
	row *types.User
}

func (s pageUsers) GetUserByID(context.Context, string) (*types.User, error) { return s.row, nil }

type pageMembers struct {
	interfaces.TenantMemberService
	row *types.TenantMember
}

func (s pageMembers) GetMembership(context.Context, string, uint64) (*types.TenantMember, error) {
	return s.row, nil
}

type pageAgents struct {
	interfaces.CustomAgentService
	row *types.CustomAgent
}

func (s pageAgents) GetAgentByID(context.Context, string) (*types.CustomAgent, error) {
	return s.row, nil
}
func TestPageSetupUsesCurrentHumanMembershipAndOwnership(t *testing.T) {
	user := &types.User{ID: "u", IsActive: true}
	member := &types.TenantMember{UserID: "u", TenantID: 7, Role: types.TenantRoleContributor, Status: types.TenantMemberStatusActive}
	a := &types.CustomAgent{ID: "a", TenantID: 7, CreatedBy: "u"}
	s := &LangflowCatalogService{users: pageUsers{row: user}, members: pageMembers{row: member}, agents: pageAgents{row: a}}
	ctx := types.WithCaller(context.Background(), types.Caller{UserID: "u", TenantID: 7})
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: "u"})
	if _, err := s.editableAgent(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	a.CreatedBy = "other"
	if _, err := s.editableAgent(ctx, "a"); err == nil {
		t.Fatal("contributor edited another owner's Agent")
	}
	member.Role = types.TenantRoleAdmin
	if _, err := s.editableAgent(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	member.Role = types.TenantRoleViewer
	a.CreatedBy = "u"
	if _, err := s.editableAgent(ctx, "a"); err == nil {
		t.Fatal("viewer configured execution")
	}
	member.Role = types.TenantRoleAdmin
	member.Status = types.TenantMemberStatusSuspended
	if _, err := s.editableAgent(ctx, "a"); err == nil {
		t.Fatal("suspended membership accepted")
	}
	member.Status = types.TenantMemberStatusActive
	a.TenantID = 8
	if _, err := s.editableAgent(ctx, "a"); err == nil {
		t.Fatal("foreign workspace accepted")
	}
	a.TenantID = 7
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalAPITenant, ID: "key"})
	if _, err := s.editableAgent(ctx, "a"); err == nil {
		t.Fatal("API key configured human execution")
	}
}
