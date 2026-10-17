package langflowruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type conversationModel struct{ calls [][]chat.Message }

func (m *conversationModel) GetModelName() string { return "fake" }
func (m *conversationModel) GetModelID() string   { return "m" }
func (m *conversationModel) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	return nil, fmt.Errorf("unused")
}
func (m *conversationModel) ChatStream(_ context.Context, messages []chat.Message, _ *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	m.calls = append(m.calls, append([]chat.Message(nil), messages...))
	out := make(chan types.StreamResponse, 1)
	out <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: "done", Done: true, FinishReason: "stop"}
	close(out)
	return out, nil
}
func TestMultiTurnUsesOnlyHostHistoryAndDisablesAllNativeMemory(t *testing.T) {
	id := "f0e5ba76-0ab0-4056-ae67-9073a36f1e01"
	graph := json.RawMessage(`{"nodes":[{"id":"agent","data":{"type":"Agent"}},{"id":"expert","data":{"type":"Agent"}},{"id":"input","data":{"type":"ChatInput"}},{"id":"output","data":{"type":"ChatOutput"}}],"edges":[]}`)
	digest, _ := langflowrelease.Digest(graph)
	model := &conversationModel{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{"id": id, "user_id": "runner", "data": graph})
			return
		}
		var request struct {
			Input  string                    `json:"input_value"`
			Tweaks map[string]map[string]any `json:"tweaks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		for _, n := range []string{"agent", "expert"} {
			if request.Tweaks[n]["n_messages"] != float64(0) {
				t.Error("native Agent memory remains enabled")
			}
		}
		for _, n := range []string{"input", "output"} {
			if request.Tweaks[n]["should_store_message"] != false {
				t.Error("native message storage remains enabled")
			}
		}
		fields := request.Tweaks["model"]
		body, _ := json.Marshal(map[string]any{"model": "m", "messages": []map[string]string{{"role": "system", "content": "current instructions"}, {"role": "user", "content": request.Input}}})
		for i := 0; i < 2; i++ {
			req, _ := http.NewRequest("POST", fields["bridge_url"].(string)+"/v1/chat/completions", strings.NewReader(string(body)))
			req.Header.Set("Authorization", "Bearer "+fields["task_capability"].(string))
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			if res.StatusCode != 200 {
				t.Error("model bridge failed", res.StatusCode)
			}
		}
		w.Write([]byte(`{"outputs":[{"outputs":[{"results":{"message":{"text":"done"}}}]}]}`))
	}))
	defer server.Close()
	b := langflowdef.Binding{AgentID: "a", FlowID: id, WorkspaceID: 1, Models: []string{"m"}, Nodes: []langflowdef.NodeBinding{{ID: "model", Kind: "model"}}, ReleaseID: "v1", GraphDigest: digest, RuntimeOwnerID: "runner"}
	e, err := New(b, hostcapability.Options{FlowURL: server.URL, ServiceAPIKey: "runner-key", RequireRelease: true, VerifyRelease: func(context.Context) error { return nil }, Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil }, ResolveModel: func(context.Context, string) (chat.Chat, error) { return model, nil }})
	if err != nil {
		t.Fatal(err)
	}
	history := []chat.Message{{Role: "user", Content: "项目松石"}, {Role: "assistant", Content: "收到松石"}}
	for _, run := range []struct {
		session string
		history []chat.Message
	}{{"one", history}, {"two", nil}, {"disabled", nil}} {
		if _, err = e.Execute(context.Background(), run.session, "message", "它叫什么？", run.history); err != nil {
			t.Fatal(err)
		}
	}
	if len(model.calls) != 6 {
		t.Fatal("unexpected model calls", len(model.calls))
	}
	for i, messages := range model.calls {
		expected := 2
		if i < 2 {
			expected = 4
		}
		if len(messages) != expected || messages[0].Role != "system" || messages[len(messages)-1].Content != "它叫什么？" {
			t.Fatal("incorrect or duplicated history", i, messages)
		}
		if i < 2 && messages[1].Content != "项目松石" {
			t.Fatal("host history missing")
		}
	}
}
