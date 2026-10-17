package langflowrelease

import (
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"testing"
)

func TestReleaseGraphRejectsCapabilitiesAndExternalDependencies(t *testing.T) {
	b := langflowdef.Binding{AgentID: "a", FlowID: "f", WorkspaceID: 1, Models: []string{"m"}, Nodes: []langflowdef.NodeBinding{{ID: "model", Kind: "model"}}}
	graph := map[string]any{"nodes": []any{map[string]any{"id": "model", "data": map[string]any{"type": "TeKnowraModel@key", "node": map[string]any{"template": map[string]any{"code": map[string]any{"value": "approved"}, "model_id": map[string]any{"value": "model · m"}, "task_capability": map[string]any{"value": ""}}}}}}, "edges": []any{}}
	encode := func() json.RawMessage { raw, _ := json.Marshal(graph); return raw }
	approved := map[string]string{"TeKnowraModel": "approved"}
	if err := ValidateGraph(encode(), b, approved); err != nil {
		t.Fatal(err)
	}
	node := graph["nodes"].([]any)[0].(map[string]any)
	data := node["data"].(map[string]any)
	fields := data["node"].(map[string]any)["template"].(map[string]any)
	fields["task_capability"].(map[string]any)["value"] = "secret"
	if ValidateGraph(encode(), b, approved) == nil {
		t.Fatal("saved task capability accepted")
	}
	fields["task_capability"].(map[string]any)["value"] = ""
	fields["model_id"].(map[string]any)["value"] = "foreign"
	if ValidateGraph(encode(), b, approved) == nil {
		t.Fatal("unbound model accepted")
	}
	fields["model_id"].(map[string]any)["value"] = "m"
	fields["model_id"].(map[string]any)["load_from_db"] = true
	if ValidateGraph(encode(), b, approved) == nil {
		t.Fatal("global dependency accepted")
	}
	fields["model_id"].(map[string]any)["load_from_db"] = false
	fields["is_refresh"] = false
	if err := ValidateGraph(encode(), b, approved); err != nil {
		t.Fatal("official refresh flag rejected:", err)
	}
	data["type"] = "RunFlow"
	if ValidateGraph(encode(), b, approved) == nil {
		t.Fatal("external flow accepted")
	}
	data["type"] = "ext:teknowra:TeKnowraModel@key"
	if err := ValidateGraph(encode(), b, approved); err != nil {
		t.Fatal("official bundle namespace rejected:", err)
	}
	graph["nodes"] = append(graph["nodes"].([]any), map[string]any{"id": "agent", "data": map[string]any{"type": "Agent", "node": map[string]any{"template": map[string]any{"code": map[string]any{"value": "evil"}}}}})
	if ValidateGraph(encode(), b, approved) == nil {
		t.Fatal("modified native component code accepted")
	}
}
