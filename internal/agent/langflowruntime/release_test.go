package langflowruntime

import (
	"context"
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReleasedRuntimeAuthenticatesAndRejectsChangedGraph(t *testing.T) {
	id := "f0e5ba76-0ab0-4056-ae67-9073a36f1e01"
	graph := json.RawMessage(`{"nodes":[],"edges":[]}`)
	digest, _ := langflowrelease.Digest(graph)
	runCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "runner-key" {
			t.Error("unauthenticated run")
		}
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{"id": id, "user_id": "runner", "data": graph})
			return
		}
		runCount++
		w.Write([]byte(`{"outputs":[{"outputs":[{"results":{"message":{"text":"v1"}}}]}]}`))
	}))
	defer server.Close()
	b := langflowdef.Binding{AgentID: "a", FlowID: id, WorkspaceID: 1, Models: []string{"m"}, Nodes: []langflowdef.NodeBinding{{ID: "model", Kind: "model"}}, ReleaseID: "v1", GraphDigest: digest, RuntimeOwnerID: "runner"}
	o := hostcapability.Options{FlowURL: server.URL, ServiceAPIKey: "runner-key", RequireRelease: true, VerifyRelease: func(context.Context) error { return nil }, Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil }, ResolveModel: func(context.Context, string) (chat.Chat, error) { return nil, nil }}
	e, err := New(b, o)
	if err != nil {
		t.Fatal(err)
	}
	if e.RuntimeMetadata()["release_id"] != "v1" {
		t.Fatal("actual release not exposed")
	}
	if _, err = e.Execute(context.Background(), "s", "m", "query", nil); err != nil {
		t.Fatal(err)
	}
	graph = json.RawMessage(`{"nodes":[{"changed":true}],"edges":[]}`)
	if _, err = e.Execute(context.Background(), "s", "m", "query", nil); err == nil {
		t.Fatal("modified release executed")
	}
	if runCount != 1 {
		t.Fatal("tampered graph reached run endpoint")
	}
	b.ReleaseID = ""
	if _, err = New(b, o); err == nil {
		t.Fatal("unpublished draft accepted")
	}
}
