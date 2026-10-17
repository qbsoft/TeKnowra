package langflowrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowapi"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"strings"
	"testing"
)

type fakeFlows struct {
	flow langflowapi.Flow
	fail bool
}

func (f *fakeFlows) GetFlow(context.Context, string) (langflowapi.Flow, error) { return f.flow, nil }
func (f *fakeFlows) CreateFlow(_ context.Context, _ string, data json.RawMessage) (langflowapi.Flow, error) {
	if f.fail {
		return langflowapi.Flow{}, fmt.Errorf("import failed")
	}
	f.flow.Data = data
	return f.flow, nil
}
func TestPublishImportFailureDoesNotActivateAndRollbackUsesOldArtifact(t *testing.T) {
	store, _ := Open(t.TempDir() + "/store.json")
	b := langflowdef.Binding{AgentID: "a", FlowID: "draft", WorkspaceID: 1, Models: []string{"m"}, Nodes: []langflowdef.NodeBinding{{ID: "model", Kind: "model"}}}
	data := json.RawMessage(`{"nodes":[{"id":"model","data":{"type":"TeKnowraModel@key","node":{"template":{"code":{"value":"approved"},"model_id":{"value":"m"}}}}}],"edges":[]}`)
	editor := &fakeFlows{flow: langflowapi.Flow{ID: "draft", UserID: "editor", Data: data}}
	runtime := &fakeFlows{flow: langflowapi.Flow{ID: "runtime-v1", UserID: "runner"}}
	manager := Manager{Store: store, Editor: editor, Runtime: runtime, RuntimeOwnerID: "runner"}
	components := Components{Digest: strings.Repeat("a", 64), Approved: map[string]string{"TeKnowraModel": "approved"}}
	first, err := manager.Publish(context.Background(), b, "V1", components, strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	runtime.fail = true
	if _, err = manager.Publish(context.Background(), b, "V2", components, strings.Repeat("b", 64)); err == nil {
		t.Fatal("failed import accepted")
	}
	active, _ := store.Active("a")
	if active.ID != first.ID || len(store.List("a")) != 1 {
		t.Fatal("failed release changed active store")
	}
	runtime.fail = false
	runtime.flow.ID = "runtime-v2"
	manager.BeforeActivate = func(context.Context, Release) error { return fmt.Errorf("editor permission revoked during import") }
	if _, err = manager.Publish(context.Background(), b, "V2", components, strings.Repeat("b", 64)); err == nil {
		t.Fatal("revoked publisher activated release")
	}
	active, _ = store.Active("a")
	if active.ID != first.ID {
		t.Fatal("revocation changed active version")
	}
}
