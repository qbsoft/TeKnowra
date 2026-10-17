package service

import (
	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestEinoMultiAgentMixedCapabilities(t *testing.T) {
	d, ok := einodef.Lookup("business-review-multiagent-v1")
	if !ok {
		t.Fatal("multiagent definition is not published")
	}
	no := false
	c := &types.AgentConfig{MemoryEnabled: &no, MCPSelectionMode: "selected", MCPServices: []string{"crm"}, KnowledgeBases: []string{"kb"}, SearchTargets: types.SearchTargets{&types.SearchTarget{KnowledgeBaseID: "kb"}}, AllowedTools: []string{tools.ToolSearchKnowledge, tools.AgentMCPToolKey("crm", "find_customer"), tools.AgentMCPToolKey("crm", "get_customer_credit_control"), tools.AgentMCPToolKey("crm", "check_customer_access")}}
	if err := validateEinoDefinitionConfig(d, c); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*types.AgentConfig){
		func(v *types.AgentConfig) { v.AllowedTools = append(append([]string{}, v.AllowedTools...), "write") },
		func(v *types.AgentConfig) { v.MCPServices = []string{"other"} },
		func(v *types.AgentConfig) { v.SearchTargets = nil },
		func(v *types.AgentConfig) { v.SkillsEnabled = true },
	} {
		x := *c
		mutate(&x)
		if validateEinoDefinitionConfig(d, &x) == nil {
			t.Fatal("undeclared capability accepted")
		}
	}
}
