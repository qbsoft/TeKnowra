package service

import (
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/langflowcatalog"
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"github.com/Tencent/WeKnora/internal/types"
	"os"
	"testing"
)

func TestPageStarterTemplateCarriesSelectedResourcesAndValidEdges(t *testing.T) {
	raw, err := os.ReadFile("../../../integrations/langflow/templates/agent-components.json")
	if err != nil {
		t.Fatal(err)
	}
	c := langflowcatalog.Catalog{AgentID: "a", Models: []langflowcatalog.Resource{{ID: "m", Name: "模型"}}, KnowledgeBases: []langflowcatalog.Resource{{ID: "kb", Name: "制度"}}, Skills: []langflowcatalog.SkillResource{{ID: "review", Name: "技能"}}, Services: []langflowcatalog.Service{{ID: "svc", Name: "MCP"}}}
	cfg := types.CustomAgentConfig{ExecutionEngine: "langflow", AgentMode: types.AgentModeSmartReasoning, ModelID: "m", SystemPrompt: "使用授权工具查制度", KBSelectionMode: "selected", KnowledgeBases: []string{"kb"}, MCPSelectionMode: "selected", MCPServices: []string{"svc"}, AllowedTools: []string{"mcp:svc:lookup"}, SkillsSelectionMode: "selected", SelectedSkills: []string{"review"}, SandboxConfigID: "box"}
	graph, err := buildPageStarterGraph(raw, c, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := pageLangflowBinding(&types.CustomAgent{ID: "a", TenantID: 7, Config: cfg}, "flow", graph, nil)
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Nodes map[string]struct {
			Data struct {
				Node struct {
					Template map[string]json.RawMessage `json:"template"`
				} `json:"node"`
			} `json:"data"`
		} `json:"nodes"`
	}
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	approved := map[string]string{}
	for kind, n := range source.Nodes {
		var code struct {
			Value string `json:"value"`
		}
		_ = json.Unmarshal(n.Data.Node.Template["code"], &code)
		approved[kind] = code.Value
	}
	if err = langflowrelease.ValidateGraph(graph, b, approved); err != nil {
		t.Fatal("template graph cannot be published:", err)
	}
	b.KnowledgeBases = nil
	if langflowrelease.ValidateGraph(graph, b, approved) == nil {
		t.Fatal("graph bypassed saved knowledge authority")
	}
}
