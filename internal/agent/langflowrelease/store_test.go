package langflowrelease

import (
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"path/filepath"
	"strings"
	"testing"
)

func testRelease(id string) Release {
	return Release{ID: id, AgentID: "a", WorkspaceID: 1, EditorFlowID: "draft", RuntimeOwnerID: "runner", GraphDigest: strings.Repeat("a", 64), ToolsDigest: strings.Repeat("b", 64), ComponentsDigest: strings.Repeat("c", 64), Binding: langflowdef.Binding{AgentID: "a", FlowID: "runtime-" + id, WorkspaceID: 1, Models: []string{"m"}, Nodes: []langflowdef.NodeBinding{{ID: "model", Kind: "model"}}}}
}
func TestReleaseStorePinsVersionAcrossActivationAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "releases.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Add(testRelease("v1")); err != nil {
		t.Fatal(err)
	}
	if err = store.Activate("a", "v1"); err != nil {
		t.Fatal(err)
	}
	pinned, _ := store.Active("a")
	if err = store.Add(testRelease("v2")); err != nil {
		t.Fatal(err)
	}
	if err = store.Activate("a", "v2"); err != nil {
		t.Fatal(err)
	}
	current, _ := store.Active("a")
	if current.ID != "v2" || pinned.Binding.FlowID != "runtime-v1" {
		t.Fatal("task version changed")
	}
	current.Binding.Models[0] = "changed"
	reread, _ := store.Active("a")
	if reread.Binding.Models[0] != "m" {
		t.Fatal("release alias")
	}
	if err = store.Activate("foreign", "v1"); err == nil {
		t.Fatal("cross-agent activation accepted")
	}
	if err = store.Add(testRelease("v1")); err == nil {
		t.Fatal("release overwritten")
	}
	restored, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := restored.Active("a")
	if r.ID != "v2" {
		t.Fatal("active version not persisted")
	}
	if err = restored.Activate("a", "v1"); err != nil {
		t.Fatal(err)
	}
	r, _ = restored.Active("a")
	if r.ID != "v1" {
		t.Fatal("rollback failed")
	}
}
