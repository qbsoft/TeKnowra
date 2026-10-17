package langflowruntime

import (
	"context"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"net/http/httptest"
	"strings"
	"testing"
)

type thinkingModel struct {
	conversationModel
	options *chat.ChatOptions
}

func (m *thinkingModel) GetModelName() string { return "deepseek-v4-flash" }
func (m *thinkingModel) ChatStream(ctx context.Context, messages []chat.Message, opts *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	m.options = opts
	return m.conversationModel.ChatStream(ctx, messages, opts)
}
func TestBridgePreservesHostThinkingWithoutIncreasingBudget(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		m := &thinkingModel{}
		b := &bridge{binding: langflowdef.Binding{Models: []string{"m"}}, opts: hostcapability.Options{Config: &types.AgentConfig{Thinking: &enabled}, ResolveModel: func(context.Context, string) (chat.Chat, error) { return m, nil }, Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil }}}
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"test"}],"max_tokens":4096}`))
		if err := b.model(context.Background(), httptest.NewRecorder(), req); err != nil {
			t.Fatal(err)
		}
		if m.options.Thinking == nil || *m.options.Thinking != enabled || m.options.ThinkingControl != "thinking_type" || m.options.CompletionBudget() != 4096 {
			t.Fatalf("host reasoning settings dropped or budget changed: %+v", m.options)
		}
	}
}

type truncatedModel struct {
	conversationModel
	calls int
}

func (m *truncatedModel) ChatStream(context.Context, []chat.Message, *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	m.calls++
	out := make(chan types.StreamResponse, 1)
	out <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: "partial", Done: true, FinishReason: "length"}
	close(out)
	return out, nil
}
func TestBridgeNeverRetriesTruncatedOutputOrAcceptsCanvasThinkingOverride(t *testing.T) {
	m := &truncatedModel{}
	b := &bridge{binding: langflowdef.Binding{Models: []string{"m"}}, opts: hostcapability.Options{ResolveModel: func(context.Context, string) (chat.Chat, error) { return m, nil }, Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil }}}
	request := func(extra string) error {
		return b.model(context.Background(), httptest.NewRecorder(), httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"test"}]`+extra+`}`)))
	}
	if err := request(""); err == nil || m.calls != 1 {
		t.Fatal("truncated output accepted or silently retried")
	}
	if err := request(`,"thinking":{"type":"enabled"}`); err == nil || m.calls != 1 {
		t.Fatal("canvas overrode host reasoning settings")
	}
}
