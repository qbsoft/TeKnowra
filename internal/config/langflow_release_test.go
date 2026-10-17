package config

import (
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"strings"
	"testing"
)

func TestLangflowDraftRemainsSeparateFromActiveRelease(t *testing.T) {
	c := &Config{}
	if err := c.loadLangflowBindings([]byte(`{"bindings":[{"agent_id":"a","flow_id":"draft","workspace_id":1,"models":["m"],"nodes":[{"id":"model","kind":"model"}]}]}`)); err != nil {
		t.Fatal(err)
	}
	store, err := langflowrelease.Open(t.TempDir() + "/release.json")
	if err != nil {
		t.Fatal(err)
	}
	c.langflowReleases = store
	draft, _ := c.DraftLangflowBinding("a")
	released := draft.Clone()
	released.FlowID = "runtime-v1"
	err = store.Add(langflowrelease.Release{ID: "v1", AgentID: "a", WorkspaceID: 1, EditorFlowID: "draft", RuntimeOwnerID: "runner", GraphDigest: strings.Repeat("a", 64), ToolsDigest: strings.Repeat("b", 64), ComponentsDigest: strings.Repeat("c", 64), Binding: released})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Activate("a", "v1"); err != nil {
		t.Fatal(err)
	}
	runtime, _ := c.LangflowBinding("a")
	again, _ := c.DraftLangflowBinding("a")
	if runtime.FlowID != "runtime-v1" || runtime.ReleaseID != "v1" || again.FlowID != "draft" {
		t.Fatal("editor/runtime binding mixed")
	}
}
