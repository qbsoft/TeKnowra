package session

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type abandonedMessages struct {
	interfaces.MessageService
	current *types.Message
	saved   *types.Message
	saveErr error
}

func (s *abandonedMessages) GetMessage(context.Context, string, string) (*types.Message, error) {
	return s.current, nil
}
func (s *abandonedMessages) UpdateMessage(_ context.Context, m *types.Message) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	copy := *m
	s.saved = &copy
	return nil
}

type abandonedStream struct {
	*einoStreamCapture
	live      string
	lookupErr error
}

func (s *abandonedStream) GetLiveRun(context.Context, string) (string, string, error) {
	return s.live, "", s.lookupErr
}

func TestEinoAbandonedRunEndsExplicitly(t *testing.T) {
	for _, tc := range []struct {
		name, live                                string
		completed, ordinary, lookupFail, saveFail bool
	}{
		{name: "process_lost"}, {name: "live", live: "m"}, {name: "finished_race", completed: true},
		{name: "ordinary", ordinary: true}, {name: "lookup_failure", lookupFail: true}, {name: "save_failure", saveFail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := &types.Message{ID: "m", SessionID: "s"}
			fresh := &types.Message{ID: "m", SessionID: "s", IsCompleted: tc.completed, Content: ""}
			messages := &abandonedMessages{current: fresh}
			capture := &abandonedStream{einoStreamCapture: &einoStreamCapture{}, live: tc.live}
			if tc.lookupFail {
				capture.lookupErr = errors.New("redis unavailable")
			}
			if tc.saveFail {
				messages.saveErr = errors.New("database unavailable")
			}
			events := []interfaces.StreamEvent{{Type: types.ResponseTypeThinking, Data: map[string]interface{}{"definition_version": "customer-credit-readonly-v1", "run_id": "r", "node_key": "credit"}}}
			if tc.ordinary {
				events[0].Data = nil
			}
			h := &Handler{messageService: messages, streamManager: capture}
			ended, err := h.finishAbandonedEinoRun(context.Background(), message, events)
			if tc.lookupFail || tc.saveFail {
				if err == nil || ended || len(capture.events) != 0 {
					t.Fatal("storage failure announced termination")
				}
				return
			}
			if tc.live != "" || tc.completed || tc.ordinary {
				if ended || messages.saved != nil || len(capture.events) != 0 {
					t.Fatal("live or completed or ordinary run changed")
				}
				return
			}
			if !ended || err != nil || messages.saved == nil || !messages.saved.IsCompleted || messages.saved.Content != "任务未完成：执行进程已中断，无法继续本次任务，请重新发起。" {
				t.Fatalf("orphan not explicitly ended: saved=%+v err=%v", messages.saved, err)
			}
			if len(capture.events) != 2 || capture.events[0].Type != types.ResponseTypeError || capture.events[1].Type != types.ResponseTypeComplete {
				t.Fatal("missing failure followed by stream termination")
			}
		})
	}
}

func TestDualEngineAbandonedNewDefinitions(t *testing.T) {
	for _, definition := range []string{"knowledge-summary-v1", "business-review-multiagent-v1", "langflow-release"} {
		t.Run(definition, func(t *testing.T) {
			message := &types.Message{ID: "m", SessionID: "s"}
			messages := &abandonedMessages{current: message}
			capture := &abandonedStream{einoStreamCapture: &einoStreamCapture{}}
			events := []interfaces.StreamEvent{{Type: types.ResponseTypeThinking, Data: map[string]interface{}{"definition_version": definition, "execution_engine": "langflow", "run_id": "r"}}}
			h := &Handler{messageService: messages, streamManager: capture}
			ended, err := h.finishAbandonedEinoRun(context.Background(), message, events)
			if err != nil || !ended || messages.saved == nil || !messages.saved.IsCompleted {
				t.Fatalf("lost %s run not settled: ended=%v err=%v", definition, ended, err)
			}
		})
	}
}

func TestDualEngineAbandonedSnapshotBeforeFirstEvent(t *testing.T) {
	for _, engine := range []string{"eino", "langflow"} {
		t.Run(engine, func(t *testing.T) {
			message := &types.Message{ID: "m", SessionID: "s", ExecutionContext: types.MessageExecutionContext{ExecutionEngine: engine, ExecutionOwner: "previous-process"}}
			messages := &abandonedMessages{current: message}
			capture := &abandonedStream{einoStreamCapture: &einoStreamCapture{}}
			h := &Handler{messageService: messages, streamManager: capture}
			ended, err := h.finishAbandonedEinoRun(context.Background(), message, nil)
			if err != nil || !ended || len(message.ExecutionContext.ExecutionDiagnostics) != 1 {
				t.Fatalf("snapshot not settled: %v %v", ended, err)
			}
			if message.ExecutionContext.ExecutionDiagnostics[0].Engine != engine {
				t.Fatal("engine changed")
			}
		})
	}
}

func TestDualEngineAbandonedPreservesDurableInputWait(t *testing.T) {
	message := &types.Message{ID: "m", SessionID: "s", RunState: types.MessageRunStateAwaitingInput, ExecutionContext: types.MessageExecutionContext{ExecutionEngine: "langflow"}}
	h := &Handler{}
	ended, err := h.finishAbandonedEinoRun(context.Background(), message, nil)
	if err != nil || ended || message.IsCompleted {
		t.Fatal("durable input wait destroyed")
	}
}

func TestDualEngineAbandonedUnownedBeforeMarkerIsNotProofOfCrash(t *testing.T) {
	message := &types.Message{ID: "m", SessionID: "s", ExecutionContext: types.MessageExecutionContext{ExecutionEngine: "langflow"}}
	messages := &abandonedMessages{current: message}
	capture := &abandonedStream{einoStreamCapture: &einoStreamCapture{}}
	h := &Handler{messageService: messages, streamManager: capture}
	ended, err := h.finishAbandonedEinoRun(context.Background(), message, nil)
	if err != nil || ended || messages.saved != nil {
		t.Fatal("pre-marker publication window was mistaken for a crash")
	}
}

func TestDualEngineAbandonedSameProcessBeforeMarker(t *testing.T) {
	message := &types.Message{ID: "m", SessionID: "s", ExecutionContext: types.MessageExecutionContext{ExecutionEngine: "langflow", ExecutionOwner: executionProcessID}}
	h := &Handler{}
	ended, err := h.finishAbandonedEinoRun(context.Background(), message, nil)
	if err != nil || ended || message.IsCompleted {
		t.Fatal("active process publication window destroyed")
	}
}
