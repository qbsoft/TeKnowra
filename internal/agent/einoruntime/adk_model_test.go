package einoruntime

import (
	"context"
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"testing"
)

type metadataModel struct {
	fixtureChat
	messages []chat.Message
	options  *chat.ChatOptions
}

func (*metadataModel) GetModelName() string { return "deepseek-v4-flash" }
func (f *metadataModel) ChatStream(ctx context.Context, in []chat.Message, o *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	f.messages = in
	f.options = o
	return f.fixtureChat.ChatStream(ctx, in, o)
}
func TestADKProviderStateRoundTripAndBoundOptions(t *testing.T) {
	base, _, _ := reviewFixture(t)
	f := &metadataModel{fixtureChat: fixtureChat{events: []types.StreamResponse{{ResponseType: types.ResponseTypeThinking, Content: "reason"}, {ResponseType: types.ResponseTypeAnswer, Done: true, FinishReason: "tool_calls", ToolCalls: []types.LLMToolCall{{ID: "one", Type: "function", Function: types.FunctionCall{Name: "contract_expert", Arguments: `{}`}, ProviderMetadata: types.ToolCallMetadata{"signature": json.RawMessage(`"provider-state"`)}}}}, {Usage: &types.TokenUsage{TotalTokens: 7}}}}}
	base.opts.Model = f
	no := false
	base.opts.Config.Thinking = &no
	r := &run{engine: base, state: &types.AgentState{}, usageComplete: true, expertResults: map[string]string{}}
	ctx := context.WithValue(context.Background(), runKey{}, r)
	adapter := &taskChatModel{run: r, agent: "test"}
	bound, err := adapter.WithTools([]*schema.ToolInfo{{Name: "contract_expert", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := bound.Generate(ctx, []*schema.Message{schema.UserMessage("synthetic")})
	if err != nil {
		t.Fatal(err)
	}
	if len(adapter.tools) != 0 || len(f.options.Tools) != 1 || f.options.ThinkingControl != "thinking_type" || f.options.Thinking == nil || *f.options.Thinking || out.ReasoningContent != "reason" || r.state.TurnUsage.TotalTokens != 7 {
		t.Fatal("host model state/options lost")
	}
	f.events = []types.StreamResponse{{ResponseType: types.ResponseTypeAnswer, Content: "done", Done: true, FinishReason: "stop"}}
	_, err = bound.Generate(ctx, []*schema.Message{out, schema.UserMessage("next")})
	if err != nil {
		t.Fatal(err)
	}
	if f.messages[0].ReasoningContent != "reason" || string(f.messages[0].ToolCalls[0].ProviderMetadata["signature"]) != `"provider-state"` {
		t.Fatal("provider state did not round trip")
	}
	if _, err = bound.Generate(ctx, []*schema.Message{schema.UserMessage("test")}, model.WithMaxTokens(2048)); err == nil {
		t.Fatal("token ceiling overridden")
	}
}
