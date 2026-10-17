package langflowruntime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFlowWirePreservesToolDescriptionAndSchema(t *testing.T) {
	description := "first line\nquoted \"customer_id\" and \\ path"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auto_login" {
			w.WriteHeader(200)
			return
		}
		var request struct {
			Tweaks map[string]map[string]any `json:"tweaks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		wire, _ := request.Tweaks["tool-node"]["tools_json"].(string)
		if !strings.HasPrefix(wire, "base64:") {
			t.Error("tool metadata exposed to text unescaping")
			w.WriteHeader(500)
			return
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(wire, "base64:"))
		var tools []hostcapability.ToolDescriptor
		if err != nil || json.Unmarshal(raw, &tools) != nil || len(tools) != 1 || tools[0].Description != description {
			t.Error("tool metadata changed in flow transport")
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"outputs":[{"outputs":[{"results":{"message":{"text":"done"}}}]}]}`))
	}))
	defer server.Close()
	binding := langflowdef.Binding{AgentID: "a", FlowID: "f", WorkspaceID: 1, Models: []string{"m"}, Tools: []langflowdef.ToolBinding{{ServiceID: "s", Name: "lookup"}}, Nodes: []langflowdef.NodeBinding{{ID: "model-node", Kind: "model"}, {ID: "tool-node", Kind: "mcp"}}}
	opts := hostcapability.Options{FlowURL: server.URL, LocalAutoLogin: true, Tools: []hostcapability.ToolDescriptor{{ServiceID: "s", Name: "lookup", Description: description, InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}}}`)}}, Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil }, ResolveModel: func(context.Context, string) (chat.Chat, error) { return nil, nil }, CallTool: func(context.Context, hostcapability.Invocation) (map[string]any, error) { return nil, nil }}
	engine, err := New(binding, opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = engine.Execute(context.Background(), "session", "message", "readonly", nil); err != nil {
		t.Fatal(err)
	}
}

func TestToolRoutesByServiceAndRetainsStringID(t *testing.T) {
	var got hostcapability.Invocation
	b := &bridge{ctx: context.Background(), token: "secret", binding: langflowdef.Binding{Tools: []langflowdef.ToolBinding{{ServiceID: "one", Name: "lookup"}, {ServiceID: "two", Name: "lookup"}}}, opts: hostcapability.Options{
		Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil }, CallTool: func(_ context.Context, in hostcapability.Invocation) (map[string]any, error) {
			got = in
			return map[string]any{"ok": true}, nil
		}}}
	req := httptest.NewRequest("POST", "/tool", strings.NewReader(`{"service_id":"two","tool":"lookup","args":{"customer_id":"2075179812203560962"}}`))
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	b.ServeHTTP(w, req)
	if w.Code != 200 || got.ServiceID != "two" || got.Args["customer_id"] != "2075179812203560962" {
		t.Fatalf("route: %d %#v", w.Code, got)
	}
}
func TestToolRejectsUndeclaredServiceAndIdentity(t *testing.T) {
	b := &bridge{ctx: context.Background(), token: "secret", binding: langflowdef.Binding{Tools: []langflowdef.ToolBinding{{ServiceID: "one", Name: "lookup"}}}}
	for _, raw := range []string{`{"service_id":"foreign","tool":"lookup","args":{}}`, `{"service_id":"one","tool":"lookup","args":{},"user_id":"admin"}`} {
		req := httptest.NewRequest("POST", "/tool", strings.NewReader(raw))
		req.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		b.ServeHTTP(w, req)
		if w.Code == 200 {
			t.Fatal("unpublished identity accepted")
		}
	}
}
func TestRuntimeRequiresOnlyDeclaredCapabilities(t *testing.T) {
	base := langflowdef.Binding{AgentID: "a", FlowID: "f", WorkspaceID: 1, Models: []string{"m"}, Nodes: []langflowdef.NodeBinding{{ID: "m", Kind: "model"}}}
	o := hostcapability.Options{Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil }, ResolveModel: nil}
	if _, err := New(base, o); err == nil {
		t.Fatal("missing model resolver accepted")
	}
	o.ResolveModel = func(context.Context, string) (chat.Chat, error) { return nil, nil }
	if _, err := New(base, o); err != nil {
		t.Fatalf("model-only configuration required business callbacks: %v", err)
	}
}
