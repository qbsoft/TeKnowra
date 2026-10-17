package session

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"strings"
	"testing"
	"time"
)

func TestAllPublishedEinoFailuresRemainVisibleAfterReload(t *testing.T) {
	for _, definition := range einodef.IDs() {
		for _, cancelled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cancelled=%t", definition, cancelled), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if cancelled {
					cancel()
				}
				capture := &einoStreamCapture{}
				message := &types.Message{ID: "m"}
				h := &AgentStreamHandler{ctx: ctx, assistantMessage: message, streamManager: capture, einoDefinitionVersion: definition}
				steps := []types.AgentStep{{NodeKey: "validate", RunID: "run", DefinitionVersion: definition}}
				if err := h.handleError(ctx, event.Event{Data: event.ErrorData{Error: "事实复核拒绝发布", Extra: map[string]interface{}{"agent_steps": steps}}}); err != nil {
					t.Fatal(err)
				}
				raw, _ := json.Marshal(message)
				var restored types.Message
				if err := json.Unmarshal(raw, &restored); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(restored.Content, "未完成：事实复核拒绝发布") || len(restored.AgentSteps) != 1 || restored.AgentSteps[0].RunID != "run" {
					t.Fatalf("published definition lost failure: %+v", restored)
				}
				if cancelled && len(capture.events) != 0 {
					t.Fatal("cancelled task emitted live error")
				}
			})
		}
	}
}

type einoStreamCapture struct {
	interfaces.StreamManager
	events []interfaces.StreamEvent
}

func (s *einoStreamCapture) AppendEvent(_ context.Context, _, _ string, e interfaces.StreamEvent) error {
	s.events = append(s.events, e)
	return nil
}
func TestEinoNodeMetadataSurvivesHandlerAndSSE(t *testing.T) {
	capture := &einoStreamCapture{}
	message := &types.Message{ID: "m"}
	h := &AgentStreamHandler{ctx: context.Background(), sessionID: "s", assistantMessageID: "m", assistantMessage: message, streamManager: capture, eventStartTimes: map[string]time.Time{}}
	for _, done := range []bool{false, true} {
		if err := h.handleThought(context.Background(), event.Event{ID: "run:plan", Type: event.EventAgentThought, Data: event.AgentThoughtData{Content: "整理审查问题", Done: done, RunID: "run", NodeKey: "plan", DefinitionVersion: "contract-review-v1"}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(capture.events) != 2 {
		t.Fatal("missing start or completion")
	}
	if len(message.AgentSteps) != 1 || message.AgentSteps[0].NodeKey != "plan" {
		t.Fatal("partial progress not available before completion")
	}
	for _, ev := range capture.events {
		raw, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		var restored interfaces.StreamEvent
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		response := buildStreamResponse(restored, "request")
		for key, want := range map[string]string{"run_id": "run", "node_key": "plan", "definition_version": "contract-review-v1", "event_id": "run:plan"} {
			if response.Data[key] != want {
				t.Fatalf("lost %s: %+v", key, response)
			}
		}
	}
	stored := tools.SanitizeAgentStepsForStorage([]types.AgentStep{{Thought: "整理审查问题", RunID: "run", NodeKey: "plan", DefinitionVersion: "contract-review-v1"}})
	if stored[0].RunID != "run" || stored[0].NodeKey != "plan" || stored[0].DefinitionVersion != "contract-review-v1" {
		t.Fatal("persist sanitizer lost identity")
	}
}

func TestEinoFailureRetainsStepsAndErrorForReload(t *testing.T) {
	capture := &einoStreamCapture{}
	message := &types.Message{ID: "m"}
	h := &AgentStreamHandler{ctx: context.Background(), sessionID: "s", assistantMessageID: "m", assistantMessage: message, streamManager: capture}
	steps := []types.AgentStep{{Thought: "检索授权合同与政策", RunID: "run", NodeKey: "retrieve", DefinitionVersion: "contract-review-v1"}}
	err := h.handleError(context.Background(), event.Event{Type: event.EventError, Data: event.ErrorData{Error: "知识授权已失效", Stage: "agent_execution", Extra: map[string]interface{}{"definition_version": "contract-review-v1", "agent_steps": steps}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded types.Message
	if err = json.Unmarshal(raw, &reloaded); err != nil {
		t.Fatal(err)
	}
	if reloaded.Content != "审查未完成：知识授权已失效" || len(reloaded.AgentSteps) != 1 || reloaded.AgentSteps[0].NodeKey != "retrieve" {
		t.Fatalf("failure lost on reload: %+v", reloaded)
	}
}

func TestEinoCancelledFailureIsStoredWithoutLiveError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	capture := &einoStreamCapture{}
	message := &types.Message{ID: "m", AgentSteps: []types.AgentStep{{Thought: "整理审查问题", NodeKey: "plan"}}}
	h := &AgentStreamHandler{ctx: ctx, assistantMessage: message, streamManager: capture, einoDefinitionVersion: "contract-review-v1"}
	if err := h.handleError(ctx, event.Event{Data: event.ErrorData{Error: "审查已停止"}}); err != nil {
		t.Fatal(err)
	}
	if message.Content != "审查未完成：审查已停止" || len(message.AgentSteps) != 1 || len(capture.events) != 0 {
		t.Fatalf("cancel persistence incorrect: %+v, events=%d", message, len(capture.events))
	}
}

func TestCreditFailureRetainsReasonForReload(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			cancel()
		}
		capture := &einoStreamCapture{}
		message := &types.Message{ID: "m"}
		h := &AgentStreamHandler{ctx: ctx, assistantMessage: message, streamManager: capture}
		err := h.handleError(ctx, event.Event{Data: event.ErrorData{Error: "任务已停止", Extra: map[string]interface{}{"definition_version": "customer-credit-readonly-v1", "agent_steps": []types.AgentStep{{NodeKey: "customer", DefinitionVersion: "customer-credit-readonly-v1"}}}}})
		cancel()
		if err != nil || message.Content != "任务未完成：任务已停止" || len(message.AgentSteps) != 1 {
			t.Fatalf("credit failure unavailable after reload: %+v, err=%v", message, err)
		}
		if cancelled && len(capture.events) != 0 {
			t.Fatal("cancelled stream received live error")
		}
	}
}
