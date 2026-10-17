package einoruntime

import (
	"context"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"strings"
	"testing"
)

type nativeChat = chat.Chat
type fixtureChat struct {
	nativeChat
	events []types.StreamResponse
}

func (f *fixtureChat) ChatStream(ctx context.Context, _ []chat.Message, _ *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	out := make(chan types.StreamResponse)
	go func() {
		defer close(out)
		for _, ev := range f.events {
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
func TestReadModelTrailingUsageAndReasoning(t *testing.T) {
	f := &fixtureChat{events: []types.StreamResponse{{ResponseType: types.ResponseTypeThinking, Content: "reason"}, {ResponseType: types.ResponseTypeAnswer, Content: "answer", Done: true}, {Usage: &types.TokenUsage{TotalTokens: 9}, FinishReason: "stop"}}}
	r, err := ReadModel(context.Background(), f, nil, nil)
	if err != nil || r.Content != "answer" || r.ReasoningContent != "reason" || r.Usage.TotalTokens != 9 {
		t.Fatalf("r=%+v err=%v", r, err)
	}
}
func TestReadModelErrorIsNotSuccess(t *testing.T) {
	f := &fixtureChat{events: []types.StreamResponse{{ResponseType: types.ResponseTypeAnswer, Content: "partial"}, {ResponseType: types.ResponseTypeError, Content: "disconnect"}}}
	_, err := ReadModel(context.Background(), f, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "disconnect") {
		t.Fatal("error became success")
	}
}
func TestReadModelCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ReadModel(ctx, &fixtureChat{}, nil, nil)
	if err != context.Canceled {
		t.Fatalf("err=%v", err)
	}
}

func TestReadModelTruncatedJSONIsNotSuccess(t *testing.T) {
	f := &fixtureChat{events: []types.StreamResponse{{ResponseType: types.ResponseTypeAnswer, Content: `{"queries":["valid"]}`}}}
	if _, err := ReadModel(context.Background(), f, nil, nil); err == nil {
		t.Fatal("stream without terminal event accepted")
	}
}

func TestReadModelLengthFailureIncludesSafeDiagnostics(t *testing.T) {
	f := &fixtureChat{events: []types.StreamResponse{{ResponseType: types.ResponseTypeAnswer, Content: "partial", Done: true, FinishReason: "length", Usage: &types.TokenUsage{CompletionTokens: 1024}}}}
	_, err := ReadModel(context.Background(), f, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "finish=length") || !strings.Contains(err.Error(), "completion_tokens=1024") || strings.Contains(err.Error(), "partial") {
		t.Fatalf("unsafe or uninformative diagnostics: %v", err)
	}
}
