package service

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/tools"
)

const (
	mailSvc = "6a1e8a53-0000-4000-8000-000000000001"
	crmSvc  = "6a1e8a53-0000-4000-8000-000000000002"
)

// The point of the whole feature: take one tool from one service and one from
// another, leaving the rest of both behind.
func TestPredicate_PicksToolsAcrossServices(t *testing.T) {
	allow, legacy, restrict := agentMCPToolPredicate([]string{
		"thinking",
		tools.AgentMCPToolKey(mailSvc, "send_email"),
		tools.AgentMCPToolKey(crmSvc, "get_customer_profile"),
	})
	if !restrict || len(legacy) != 0 {
		t.Fatalf("restrict=%v legacy=%v, want restricting with no legacy", restrict, legacy)
	}
	for _, keep := range [][2]string{{mailSvc, "send_email"}, {crmSvc, "get_customer_profile"}} {
		if !allow(keep[0], keep[1]) {
			t.Errorf("%s/%s was denied but is granted", keep[0], keep[1])
		}
	}
	for _, gone := range [][2]string{
		{mailSvc, "selftest"},
		{crmSvc, "list_receivables"},
		{crmSvc, "send_email"}, // right tool name, wrong service
	} {
		if allow(gone[0], gone[1]) {
			t.Errorf("%s/%s was admitted but is not granted", gone[0], gone[1])
		}
	}
}

// A non-empty list that grants no MCP tool is a list that grants no MCP tool.
// The old "builtin-only list means written-before-this-shipped, skip
// filtering" compat rule is gone: config that does not mean what it says is
// worse than config that disarms an agent loudly.
func TestPredicate_ABuiltinOnlyListGrantsNoMCPTools(t *testing.T) {
	allow, _, restrict := agentMCPToolPredicate([]string{"thinking", "knowledge_search"})
	if !restrict {
		t.Fatal("a non-empty list must restrict")
	}
	if allow(mailSvc, "send_email") {
		t.Error("send_email admitted by a list that never granted it")
	}
}

func TestPredicate_EmptyConfigIsUnconfiguredNotEmptySet(t *testing.T) {
	_, _, restrict := agentMCPToolPredicate(nil)
	if restrict {
		t.Error("an empty AllowedTools was read as an empty set; it means unconfigured")
	}
}

// Registry-name grants from before the catalog rework cannot be resolved to a
// service ID. They admit nothing, but they must be reported so the operator
// learns to re-save the agent instead of debugging a silent loss.
func TestPredicate_LegacyRegistryNamesAreReportedNotHonored(t *testing.T) {
	allow, legacy, restrict := agentMCPToolPredicate([]string{
		"thinking",
		"mcp_mail_send_email",
	})
	if !restrict {
		t.Fatal("a non-empty list must restrict")
	}
	if len(legacy) != 1 || legacy[0] != "mcp_mail_send_email" {
		t.Errorf("legacy = %v, want the one old-form grant", legacy)
	}
	if allow(mailSvc, "send_email") {
		t.Error("an unresolvable legacy grant must not admit anything")
	}
}

// The grant key format is the contract between this predicate, the
// agent-tools endpoint, and the frontend checkboxes. Pin it.
func TestAgentMCPToolKeyRoundTrip(t *testing.T) {
	key := tools.AgentMCPToolKey(mailSvc, "send_email")
	if key != "mcp:"+mailSvc+":send_email" {
		t.Fatalf("key = %q", key)
	}
	svc, tool, ok := tools.ParseAgentMCPToolKey(key)
	if !ok || svc != mailSvc || tool != "send_email" {
		t.Errorf("parse(%q) = %q,%q,%v", key, svc, tool, ok)
	}
	for _, bad := range []string{"thinking", "mcp_mail_send_email", "mcp:", "mcp:onlyservice", "mcp::tool"} {
		if _, _, ok := tools.ParseAgentMCPToolKey(bad); ok {
			t.Errorf("parse(%q) accepted a non-grant", bad)
		}
	}
	// A tool name containing ':' must survive the round trip; the service ID
	// segment is a UUID so the first cut is unambiguous.
	svc, tool, ok = tools.ParseAgentMCPToolKey(tools.AgentMCPToolKey(crmSvc, "ns:reset"))
	if !ok || svc != crmSvc || tool != "ns:reset" {
		t.Errorf("colon-bearing tool name broke the round trip: %q %q %v", svc, tool, ok)
	}
}
