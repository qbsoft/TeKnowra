package langflowruntime

import (
	"context"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/types"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSkillBridgeScopesBoundSkillAndRejectsForeignNodeAndScriptGrant(t *testing.T) {
	calls := 0
	b := &bridge{ctx: context.Background(), token: "cap", binding: langflowdef.Binding{Skills: []string{"tyer-selftest"}, Nodes: []langflowdef.NodeBinding{{ID: "s", Kind: "skill"}}}, opts: hostcapability.Options{Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil }, CallSkill: func(_ context.Context, in hostcapability.SkillInvocation) (map[string]any, error) {
		calls++
		return map[string]any{"output": "skill content"}, nil
	}}}
	invoke := func(body string) int {
		r := httptest.NewRequest("POST", "/skill", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer cap")
		w := httptest.NewRecorder()
		b.ServeHTTP(w, r)
		return w.Code
	}
	if invoke(`{"node_key":"s","skill_name":"tyer-selftest","file_path":"SKILL.md"}`) != 200 || calls != 1 {
		t.Fatal("bound skill cannot load")
	}
	for _, body := range []string{`{"node_key":"s","skill_name":"foreign"}`, `{"node_key":"model","skill_name":"tyer-selftest"}`, `{"node_key":"s","skill_name":"tyer-selftest","execute":true,"file_path":"scripts/run.py"}`, `{"node_key":"s","skill_name":"tyer-selftest","command":"echo unsafe"}`} {
		if invoke(body) == 200 {
			t.Fatal("unauthorized skill capability accepted")
		}
	}
	if calls != 1 {
		t.Fatal("unauthorized call reached host")
	}
}
