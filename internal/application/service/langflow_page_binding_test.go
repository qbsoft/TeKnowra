package service

import (
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestPageBindingPreservesExistingCreditAccessGuard(t *testing.T){
 a:=&types.CustomAgent{ID:"a",TenantID:7,Config:types.CustomAgentConfig{ExecutionEngine:"langflow",AgentMode:types.AgentModeSmartReasoning,ModelID:"m",KBSelectionMode:"none",SkillsSelectionMode:"none",MCPSelectionMode:"selected",MCPServices:[]string{"crm"},AllowedTools:[]string{"mcp:crm:get_customer_credit_control"}}}
 graph:=json.RawMessage(`{"nodes":[{"id":"m","data":{"type":"TeKnowraModel"}},{"id":"t","data":{"type":"TeKnowraMCP"}}]}`)
 if _,err:=pageLangflowBinding(a,"flow",graph,nil);err==nil{t.Fatal("credit read without selected access-check tool accepted")}
 a.Config.AllowedTools=append(a.Config.AllowedTools,"mcp:crm:check_customer_access")
 b,err:=pageLangflowBinding(a,"flow",graph,nil);if err!=nil{t.Fatal(err)}
 if b.Tools[0].Guard!="crm-credit"{t.Fatal("existing host credit guard lost")}
}

func TestPageBindingUsesSavedResourcesNotGraphClaims(t *testing.T) {
	a := &types.CustomAgent{ID: "agent", TenantID: 7, Config: types.CustomAgentConfig{ExecutionEngine: "langflow", AgentMode: types.AgentModeSmartReasoning, ModelID: "m", KBSelectionMode: "selected", KnowledgeBases: []string{"kb"}, MCPSelectionMode: "none", SkillsSelectionMode: "none"}}
	graph := json.RawMessage(`{"nodes":[{"id":"model","data":{"type":"TeKnowraModel"}},{"id":"knowledge","data":{"type":"TeKnowraKnowledge"}},{"id":"agent","data":{"type":"Agent"}}]}`)
	b, err := pageLangflowBinding(a, "flow", graph, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b.WorkspaceID != 7 || len(b.Models) != 1 || b.Models[0] != "m" || len(b.KnowledgeBases) != 1 || b.KnowledgeBases[0] != "kb" || len(b.Nodes) != 2 {
		t.Fatal("saved resources not captured", b)
	}
	a.Config.KBSelectionMode = "all"
	if _, err = pageLangflowBinding(a, "flow", graph, nil); err == nil {
		t.Fatal("unbounded knowledge selection accepted")
	}
	a.Config.KBSelectionMode = "none"
	if _, err = pageLangflowBinding(a, "flow", graph, nil); err == nil {
		t.Fatal("knowledge node without grant accepted")
	}
	a.Config.KBSelectionMode = "selected"
	if _, err = pageLangflowBinding(a, "flow", json.RawMessage(`{"nodes":[{"id":"untrusted","data":{"type":"PythonCode"}}]}`), nil); err == nil {
		t.Fatal("unapproved graph type accepted")
	}
}
