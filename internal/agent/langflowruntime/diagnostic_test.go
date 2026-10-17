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

type diagnosticModel struct{ conversationModel }

func (m *diagnosticModel) ChatStream(context.Context, []chat.Message, *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	out := make(chan types.StreamResponse, 1)
	out <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: `{"approved":false,"issues":["amount mismatch"]}`, Done: true, FinishReason: "stop"}
	close(out)
	return out, nil
}
func TestReviewedBridgeCapturesRejectedAuditWithoutChangingModelResponse(t *testing.T) {
	model := &diagnosticModel{}
	b := &bridge{runID: "run", binding: langflowdef.Binding{Models: []string{"m"}, ReviewedExperts: true, ReleaseID: "release", GraphDigest: "frozen-hash"}, opts: hostcapability.Options{Config: &types.AgentConfig{}, ResolveModel: func(context.Context, string) (chat.Chat, error) { return model, nil }, Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil }}}
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"审查事实复核：check"}]}`))
	w := httptest.NewRecorder()
	if err := b.model(context.Background(), w, req); err != nil {
		t.Fatal(err)
	}
	if len(b.state.ExecutionDiagnostics) != 1 || b.state.ExecutionDiagnostics[0].Definition != "release" || b.state.ExecutionDiagnostics[0].Implementation != "frozen-hash" {
		t.Fatal("audit not tied to actual release")
	}
	if !strings.Contains(w.Body.String(), `\"approved\":false`) {
		t.Fatal("review rejection was altered")
	}
}
