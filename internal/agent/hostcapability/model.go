package hostcapability

import (
	"context"
	"fmt"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"strings"
	"time"
)

// ReadModel consumes the native stream within its node's context. There is no
// exported detached StreamReader and no polling heartbeat for Close detection.
type ModelResponse struct {
	types.ChatResponse
	UsageReported bool
}

func ReadModel(ctx context.Context, m chat.Chat, messages []chat.Message, opts *chat.ChatOptions) (*ModelResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("chat model unavailable")
	}
	if opts == nil {
		opts = &chat.ChatOptions{}
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	ch, err := m.ChatStream(child, messages, opts)
	if err != nil {
		return nil, err
	}
	drain := func() {
		cancel()
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					return
				}
			case <-timer.C:
				return
			}
		}
	}
	r := &ModelResponse{}
	var text, reason strings.Builder
	latest := map[string]types.LLMToolCall{}
	order := []string{}
	terminal := false
	for {
		select {
		case <-ctx.Done():
			drain()
			return nil, ctx.Err()
		case ev, ok := <-ch:
			if !ok {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if !terminal || r.FinishReason == types.FinishReasonIncomplete || r.FinishReason == "length" {
					return nil, fmt.Errorf("model stream ended without a complete response (finish=%s, answer_bytes=%d, reasoning_bytes=%d, completion_tokens=%d, usage_reported=%t)", r.FinishReason, text.Len(), reason.Len(), r.Usage.CompletionTokens, r.UsageReported)
				}
				// Native clients can emit Done on bare EOF and are wrapped by the
				// platform factory. Check the finish signal, never a concrete type.
				if r.FinishReason != "stop" && r.FinishReason != "tool_calls" && r.FinishReason != "end_turn" && r.FinishReason != "stop_sequence" && r.FinishReason != "tool_use" {
					return nil, fmt.Errorf("provider did not finish the model response")
				}
				r.Content = text.String()
				r.ReasoningContent = reason.String()
				for _, id := range order {
					r.ToolCalls = append(r.ToolCalls, latest[id])
				}
				return r, nil
			}
			if ev.ResponseType == types.ResponseTypeError {
				drain()
				return nil, fmt.Errorf("model stream: %s", ev.Content)
			}
			if ev.Done || ev.FinishReason != "" {
				terminal = true
			}
			if ev.ResponseType == types.ResponseTypeAnswer {
				text.WriteString(ev.Content)
			}
			if ev.ResponseType == types.ResponseTypeThinking {
				reason.WriteString(ev.Content)
			}
			if ev.Usage != nil {
				r.Usage = *ev.Usage
				r.UsageReported = true
			}
			if ev.FinishReason != "" {
				r.FinishReason = ev.FinishReason
			}
			for _, call := range ev.ToolCalls {
				if _, ok := latest[call.ID]; !ok {
					order = append(order, call.ID)
				}
				latest[call.ID] = call
			}
		}
	}
}
