package session

import (
	"context"
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"strings"
	"testing"
)

func TestExecutionDiagnosticSurvivesFailurePersistenceWithoutSSEDisclosure(t *testing.T) {
	capture := &einoStreamCapture{}
	m := &types.Message{ID: "m"}
	h := &AgentStreamHandler{ctx: context.Background(), assistantMessage: m, streamManager: capture, langflowExecution: true}
	d := types.NewExecutionDiagnostic("langflow", "validate", "fact_audit_rejected", []string{"private-review-reason"})
	if err := h.handleError(context.Background(), event.Event{Data: event.ErrorData{Error: "failed", Extra: map[string]interface{}{"execution_diagnostics": []types.ExecutionDiagnostic{d}, "agent_steps": []types.AgentStep{{ImplementationVersion: "original-source-hash"}}}}}); err != nil {
		t.Fatal(err)
	}
	raw, err := m.ExecutionContext.Value()
	if err != nil {
		t.Fatal(err)
	}
	var restored types.MessageExecutionContext
	if err = restored.Scan(raw); err != nil {
		t.Fatal(err)
	}
	if len(restored.ExecutionDiagnostics) != 1 || restored.ExecutionDiagnostics[0].Reasons[0] != "private-review-reason" {
		t.Fatal("diagnostic lost on reload")
	}
	if m.AgentSteps[0].ImplementationVersion != "original-source-hash" {
		t.Fatal("version lost")
	}
	visible, _ := json.Marshal(m)
	sse, _ := json.Marshal(capture.events)
	if strings.Contains(string(visible)+string(sse), "private-review-reason") {
		t.Fatal("private review leaked into chat")
	}
}

func TestExecutionInitializationFailureHasInternalClassification(t *testing.T) {
	for _, definition := range []string{"", "contract-review-v1"} {
		capture := &einoStreamCapture{}
		m := &types.Message{ID: "m"}
		h := &AgentStreamHandler{ctx: context.Background(), assistantMessage: m, streamManager: capture, langflowExecution: definition == "", einoDefinitionVersion: definition}
		if err := h.handleError(context.Background(), event.Event{Data: event.ErrorData{Error: "model provider api_key=sk-private", Stage: "initialization"}}); err != nil {
			t.Fatal(err)
		}
		ds := m.ExecutionContext.ExecutionDiagnostics
		if len(ds) != 1 || ds[0].Code != "model_provider_failed" || ds[0].Node != "initialization" || len(ds[0].Reasons) != 0 {
			t.Fatal("initialization failure not classified safely")
		}
	}
}
