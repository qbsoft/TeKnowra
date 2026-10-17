package service

import (
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestThirdEinoDefinitionUsesCapabilityValidation(t *testing.T) {
	d, ok := einodef.Lookup(einodef.KnowledgeSummary)
	if !ok {
		t.Fatal("third definition missing")
	}
	no := false
	c := &types.AgentConfig{AllowedTools: []string{"knowledge_search"}, MCPSelectionMode: "none", MemoryEnabled: &no}
	if err := validateEinoDefinitionConfig(d, c); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*types.AgentConfig){
		func(v *types.AgentConfig) { v.MCPServices = []string{"hidden"} },
		func(v *types.AgentConfig) { v.AllowedTools = []string{"write"} },
		func(v *types.AgentConfig) { v.SharedAgentReadOnly = true },
	} {
		copy := *c
		mutate(&copy)
		if validateEinoDefinitionConfig(d, &copy) == nil {
			t.Fatal("undeclared capability accepted")
		}
	}
}

func TestEinoBusinessResultKeepsStringIDsAndRejectsMalformedContent(t *testing.T) {
	decode := func(content any) (map[string]any, error) {
		return decodeEinoBusinessResult(&types.ToolResult{Data: map[string]any{"content_items": content}})
	}
	data, err := decode([]map[string]any{{"type": "text", "text": `{"customer_id":"2073967557021581313","creditAvailable":100.01}`}})
	if err != nil || data["customer_id"] != "2073967557021581313" || data["creditAvailable"] != json.Number("100.01") {
		t.Fatalf("business precision changed: %v %v", data, err)
	}
	for _, raw := range []string{`null`, `[]`, `{"customer_id":"9"} {"readable":true}`, `not json`} {
		if _, err := decode([]map[string]any{{"type": "text", "text": raw}}); err == nil {
			t.Fatalf("malformed business result accepted: %s", raw)
		}
	}
	if _, err := decode([]map[string]any{{"type": "image", "text": `{}`}}); err == nil {
		t.Fatal("non-text business result accepted")
	}
}

func TestEinoPilotRejectsWriteCapabilities(t *testing.T) {
	no := false
	c := &types.AgentConfig{AllowedTools: []string{"knowledge_search"}, MCPSelectionMode: "none", MemoryEnabled: &no}
	if err := validateEinoPilotConfig(c); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*types.AgentConfig){func(v *types.AgentConfig) { v.AllowedTools = append(v.AllowedTools, "submit_review_finding") }, func(v *types.AgentConfig) { v.MCPSelectionMode = "all" }, func(v *types.AgentConfig) { v.SkillsEnabled = true }, func(v *types.AgentConfig) { v.MemoryEnabled = nil }} {
		copy := *c
		mutate(&copy)
		if err := validateEinoPilotConfig(&copy); err == nil {
			t.Fatal("unsafe capability allowed")
		}
	}
}

func TestEinoCreditConfigIsExplicitReadOnly(t *testing.T) {
	no := false
	c := &types.AgentConfig{MemoryEnabled: &no, MCPSelectionMode: "selected", MCPServices: []string{"crm"}, AllowedTools: []string{tools.AgentMCPToolKey("crm", "find_customer"), tools.AgentMCPToolKey("crm", "get_customer_credit_control"), tools.AgentMCPToolKey("crm", "check_customer_access")}}
	if err := validateEinoCreditConfig(c); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*types.AgentConfig){
		func(v *types.AgentConfig) { v.SharedAgentReadOnly = true },
		func(v *types.AgentConfig) { v.MCPSelectionMode = "all" },
		func(v *types.AgentConfig) {
			v.AllowedTools = append(v.AllowedTools, tools.AgentMCPToolKey("crm", "submit_review_finding"))
		},
		func(v *types.AgentConfig) { v.MCPServices = []string{"other"} },
		func(v *types.AgentConfig) { v.MemoryEnabled = nil },
		func(v *types.AgentConfig) { v.WebSearchEnabled = true },
	} {
		copy := *c
		mutate(&copy)
		if validateEinoCreditConfig(&copy) == nil {
			t.Fatal("unsafe config accepted")
		}
	}
}

func TestEinoReadOnlyIgnoresUnusedResolvedKnowledgeScope(t *testing.T) {
	// The existing chat builder resolves workspace knowledge even for an Agent
	// with no knowledge tools. This does not grant the MCP-only executor retrieval.
	no := false
	d, _ := einodef.Lookup(einodef.CustomerCredit)
	c := &types.AgentConfig{MemoryEnabled: &no, MCPSelectionMode: "selected", MCPServices: []string{"crm"}, AllowedTools: []string{tools.AgentMCPToolKey("crm", "find_customer"), tools.AgentMCPToolKey("crm", "get_customer_credit_control"), tools.AgentMCPToolKey("crm", "check_customer_access")}, SearchTargets: types.SearchTargets{&types.SearchTarget{KnowledgeBaseID: "unused-kb"}}}
	if err := validateEinoDefinitionConfig(d, c); err != nil {
		t.Fatalf("unused resolved scope invalidated existing MCP Agent: %v", err)
	}
}
