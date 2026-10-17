package langflowrelease

import (
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"strings"
	"testing"
)

func TestPublishedSkillNodeRequiresBoundSkillAndScriptGrant(t *testing.T) {
	b := testRelease("v1").Binding
	b.Skills = []string{"tyer-selftest"}
	b.SandboxConfigID = "sandbox"
	b.Nodes = append(b.Nodes, langflowdef.NodeBinding{ID: "s", Kind: "skill"})
	graph := `{"nodes":[{"id":"model","data":{"type":"TeKnowraModel","node":{"template":{"code":{"value":"model"},"model_id":{"value":"m"}}}}},{"id":"s","data":{"type":"TeKnowraSkills","node":{"template":{"code":{"value":"skill"},"skill_name":{"value":"tyer-selftest"},"enable_scripts":{"value":false}}}}}],"edges":[]}`
	approved := map[string]string{"TeKnowraModel": "model", "TeKnowraSkills": "skill"}
	if err := ValidateGraph(json.RawMessage(graph), b, approved); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{`true`, `"true"`, `1`} {
		if ValidateGraph(json.RawMessage(strings.Replace(graph, `"value":false`, `"value":`+value, 1)), b, approved) == nil {
			t.Fatalf("ungranted or malformed script flag accepted: %s", value)
		}
	}
	b.Skills = []string{"foreign"}
	if ValidateGraph(json.RawMessage(graph), b, approved) == nil {
		t.Fatal("unbound skill published")
	}
}
