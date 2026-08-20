package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
)

// stubTool is the smallest thing the registry accepts.
type stubTool struct{ name string }

func (s *stubTool) Name() string                { return s.name }
func (s *stubTool) Description() string         { return "" }
func (s *stubTool) Parameters() json.RawMessage { return json.RawMessage(`{}`) }
func (s *stubTool) Execute(context.Context, json.RawMessage) (*types.ToolResult, error) {
	return &types.ToolResult{Success: true}, nil
}

func registryWith(names ...string) *tools.ToolRegistry {
	r := tools.NewToolRegistry()
	for _, n := range names {
		r.RegisterTool(&stubTool{name: n})
	}
	return r
}

func registryHas(r *tools.ToolRegistry, name string) bool {
	for _, n := range r.ListTools() {
		if n == name {
			return true
		}
	}
	return false
}

// The point of the whole change: take one tool from one service and one from
// another, leaving the rest of both behind.
func TestAllowlist_PicksToolsAcrossServices(t *testing.T) {
	r := registryWith(
		"thinking",
		"mcp_CRM-MCP_get_customer_profile",
		"mcp_CRM-MCP_get_customer_credit_control",
		"mcp_CRM-MCP_list_receivables",
		"mcp_mail_send_email",
		"mcp_mail_selftest",
	)
	cfg := &types.AgentConfig{AllowedTools: []string{
		"thinking",
		"mcp_CRM-MCP_get_customer_profile",
		"mcp_mail_send_email",
	}}

	applyMCPToolAllowlist(context.Background(), r, cfg)

	for _, keep := range []string{"thinking", "mcp_CRM-MCP_get_customer_profile", "mcp_mail_send_email"} {
		if !registryHas(r, keep) {
			t.Errorf("%s was dropped but is in the allowed list", keep)
		}
	}
	for _, gone := range []string{
		"mcp_CRM-MCP_get_customer_credit_control",
		"mcp_CRM-MCP_list_receivables",
		"mcp_mail_selftest",
	} {
		if registryHas(r, gone) {
			t.Errorf("%s survived but is not in the allowed list", gone)
		}
	}
}

// An agent that never mentions an MCP tool is running on a list written before
// per-tool selection existed. Filtering it would silently disarm it.
func TestAllowlist_LeavesBuiltinOnlyConfigsAlone(t *testing.T) {
	r := registryWith("thinking", "mcp_mail_send_email", "mcp_CRM-MCP_get_customer_profile")
	cfg := &types.AgentConfig{AllowedTools: []string{"thinking", "knowledge_search"}}

	applyMCPToolAllowlist(context.Background(), r, cfg)

	if len(r.ListTools()) != 3 {
		t.Errorf("tools = %v, want all three untouched", r.ListTools())
	}
}

func TestAllowlist_EmptyConfigIsUnconfiguredNotEmptySet(t *testing.T) {
	r := registryWith("thinking", "mcp_mail_send_email")

	applyMCPToolAllowlist(context.Background(), r, &types.AgentConfig{})

	if !registryHas(r, "mcp_mail_send_email") {
		t.Error("an empty AllowedTools was read as an empty set; it means unconfigured")
	}
}

// Built-ins are filtered when they are registered. Re-filtering them here would
// silently drop any that the switch registers without listing.
func TestAllowlist_NeverTouchesBuiltins(t *testing.T) {
	r := registryWith("thinking", "cronjob", "mcp_mail_send_email")
	cfg := &types.AgentConfig{AllowedTools: []string{"mcp_mail_send_email"}}

	applyMCPToolAllowlist(context.Background(), r, cfg)

	for _, builtin := range []string{"thinking", "cronjob"} {
		if !registryHas(r, builtin) {
			t.Errorf("built-in %s was dropped; built-ins are not this filter's business", builtin)
		}
	}
}

// This filter matches MCP tools by a name prefix it declares itself, so that
// the change touches no upstream file. That trade only holds while the prefix
// is right — if upstream renames it, the filter would match nothing and every
// agent would quietly regain the tools it was configured to lose.
//
// Failing here is the whole point: it turns a silent policy hole into a red
// test.
func TestMCPToolPrefixMatchesReality(t *testing.T) {
	svc := &types.MCPService{ID: "svc-1", Name: "mail"}
	real := tools.NewMCPTool(svc, &types.MCPTool{Name: "send_email"}, nil, nil, 0)

	if got := real.Name(); !strings.HasPrefix(got, mcpToolPrefix) {
		t.Fatalf("a real MCP tool is named %q, which does not start with %q — "+
			"applyMCPToolAllowlist matches nothing and silently allows every MCP tool",
			got, mcpToolPrefix)
	}
}
