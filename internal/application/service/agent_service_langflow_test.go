package service

import (
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestLangflowCapabilitiesDoNotExpandAgentScope(t *testing.T) {
	no := false
	b := langflowdef.Binding{AgentID: "a", FlowID: "f", WorkspaceID: 1, Models: []string{"m"}, KnowledgeBases: []string{"kb"}, Tools: []langflowdef.ToolBinding{{ServiceID: "one", Name: "lookup"}}}
	c := &types.AgentConfig{MemoryEnabled: &no, MCPSelectionMode: "selected", MCPServices: []string{"one"}, AllowedTools: []string{tools.AgentMCPToolKey("one", "lookup")}, SearchTargets: types.SearchTargets{&types.SearchTarget{KnowledgeBaseID: "kb", TenantID: 1}}}
	if err := validateLangflowCapabilities(b, c); err != nil {
		t.Fatal(err)
	}
	b.Tools[0].ServiceID = "foreign"
	if err := validateLangflowCapabilities(b, c); err == nil {
		t.Fatal("manifest granted foreign service")
	}
	b.Tools = nil
	b.KnowledgeBases = []string{"other"}
	if err := validateLangflowCapabilities(b, c); err == nil {
		t.Fatal("manifest granted foreign knowledge")
	}
}
func TestLangflowCapabilitiesSupportKnowledgeWithoutCRM(t *testing.T) {
	no := false
	c := &types.AgentConfig{MemoryEnabled: &no, MCPSelectionMode: "none", SearchTargets: types.SearchTargets{&types.SearchTarget{KnowledgeBaseID: "kb", TenantID: 1}}}
	b := langflowdef.Binding{KnowledgeBases: []string{"kb"}}
	if err := validateLangflowCapabilities(b, c); err != nil {
		t.Fatal(err)
	}
	c.SkillsEnabled = true
	if err := validateLangflowCapabilities(b, c); err == nil {
		t.Fatal("unsupported skills silently accepted")
	}
}
