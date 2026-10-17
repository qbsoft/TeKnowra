package langflowverify

import (
	"context"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/einoruntime"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type transientModel struct {
	calls int
	first error
}

func (m *transientModel) GetModelID() string   { return "test-model" }
func (m *transientModel) GetModelName() string { return "test-model" }
func (m *transientModel) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	return nil, fmt.Errorf("unused")
}
func (m *transientModel) ChatStream(context.Context, []chat.Message, *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	m.calls++
	if m.calls == 1 {
		return nil, m.first
	}
	c := make(chan types.StreamResponse, 1)
	c <- types.StreamResponse{Content: "verified", ResponseType: types.ResponseTypeAnswer, Done: true, FinishReason: "stop"}
	close(c)
	return c, nil
}
func TestModelRetriesOnlyTransportEOF(t *testing.T) {
	for _, tc := range []struct {
		name          string
		first         error
		status, calls int
	}{
		{"transport EOF", fmt.Errorf("send request: %w", io.EOF), 200, 2},
		{"provider rejection", fmt.Errorf("provider rejected request"), 502, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := &transientModel{first: tc.first}
			b := &bridge{ctx: context.Background(), token: "task-secret", opts: einoruntime.Options{Model: model, Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil }}}
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[{"role":"user","content":"test"}]}`))
			req.Header.Set("Authorization", "Bearer task-secret")
			res := httptest.NewRecorder()
			b.ServeHTTP(res, req)
			if res.Code != tc.status || model.calls != tc.calls {
				t.Fatalf("code=%d calls=%d", res.Code, model.calls)
			}
			if tc.status == 200 && b.failure != nil {
				t.Fatal("recovered request poisoned the task")
			}
		})
	}
}
