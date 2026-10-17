package service

import (
	"context"
	"errors"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"testing"
)

type langflowShareLookup struct {
	interfaces.AgentShareService
	agent            *types.CustomAgent
	err              error
	calls            int
	receiver, source uint64
}

func (s *langflowShareLookup) GetSharedAgentForTenant(ctx context.Context, tenant uint64, role types.TenantRole, id string, source ...uint64) (*types.CustomAgent, error) {
	s.calls++
	s.receiver = tenant
	if len(source) == 1 {
		s.source = source[0]
	}
	return s.agent, s.err
}
func TestLangflowSharedAgentReusesShareAuthorizationAndRevocation(t *testing.T) {
	b := langflowdef.Binding{AgentID: "a", WorkspaceID: 1}
	caller := types.Caller{UserID: "sales01", TenantID: 2, Role: types.TenantRoleContributor}
	lookup := &langflowShareLookup{agent: &types.CustomAgent{ID: "a", TenantID: 1}}
	service := &agentService{agentShareService: lookup}
	ctx := types.WithExecutionTenant(types.WithCaller(context.Background(), caller), 1)
	if _, err := service.loadLangflowRuntimeAgent(ctx, b, caller, true); err != nil {
		t.Fatal(err)
	}
	if lookup.receiver != 2 || lookup.source != 1 || types.CallerFromContext(ctx) != caller {
		t.Fatal("shared run confused receiver identity and resource owner")
	}
	if _, err := service.loadLangflowRuntimeAgent(ctx, b, caller, false); err == nil {
		t.Fatal("foreign run without verified shared mode accepted")
	}
	lookup.err = errors.New("share revoked")
	if _, err := service.loadLangflowRuntimeAgent(ctx, b, caller, true); err == nil {
		t.Fatal("revoked share retained task")
	}
	lookup.err = nil
	lookup.agent.TenantID = 3
	if _, err := service.loadLangflowRuntimeAgent(ctx, b, caller, true); err == nil {
		t.Fatal("wrong resource owner accepted")
	}
}
func TestLangflowLiveAuthorityUsesReceiverMembershipForVerifiedSharedAgent(t *testing.T) {
	b := langflowdef.Binding{AgentID: "a", WorkspaceID: 1}
	caller := types.Caller{UserID: "sales01", TenantID: 2}
	user := &types.User{ID: "sales01", IsActive: true}
	member := &types.TenantMember{UserID: "sales01", TenantID: 2, Role: types.TenantRoleContributor, Status: types.TenantMemberStatusActive}
	agent := &types.CustomAgent{ID: "a", TenantID: 1}
	if err := validateLangflowLiveAuthority(b, caller, user, member, agent, true); err != nil {
		t.Fatal(err)
	}
	if validateLangflowLiveAuthority(b, caller, user, member, agent) == nil {
		t.Fatal("ordinary mode accepted a foreign caller")
	}
	member.TenantID = 1
	if validateLangflowLiveAuthority(b, caller, user, member, agent, true) == nil {
		t.Fatal("owner membership substituted for receiver")
	}
}
func TestLangflowSharedCapabilitiesRetainBoundSkillAndToolScope(t *testing.T) {
	no := false
	b := langflowdef.Binding{Skills: []string{"tyer-demo"}, SandboxConfigID: "sandbox"}
	cfg := &types.AgentConfig{SharedAgentReadOnly: true, MemoryEnabled: &no, SkillsEnabled: true, SandboxConfigID: "sandbox", AllowedSkills: []string{"tyer-demo"}}
	if err := validateLangflowCapabilities(b, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.AllowedSkills = []string{"other"}
	if validateLangflowCapabilities(b, cfg) == nil {
		t.Fatal("shared mode expanded allowed skills")
	}
}
