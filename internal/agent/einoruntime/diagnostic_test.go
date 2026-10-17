package einoruntime

import (
	"context"
	"testing"
)

func TestFactRejectionCapturesBoundedDiagnosticAndImplementationWithoutPublishing(t *testing.T) {
	e, m, _ := reviewFixture(t)
	m.replies = append(m.replies[:2], `{"approved":false,"issues":["amount mismatch; api_key=sk-test-private"]}`)
	state, err := e.Execute(context.Background(), "s", "m", "review", nil)
	if err == nil || state == nil || state.IsComplete || state.FinalAnswer != "" {
		t.Fatal("rejection published")
	}
	if len(state.ExecutionDiagnostics) != 1 || state.ExecutionDiagnostics[0].Code != "fact_audit_rejected" {
		t.Fatal("specific rejection lost")
	}
	d := state.ExecutionDiagnostics[0]
	if d.Implementation != ImplementationVersion(e.opts.Definition) || d.RunID == "" {
		t.Fatal("diagnostic version/run missing")
	}
	if d.Reasons[0] != "amount mismatch; [redacted]" {
		t.Fatalf("unsafe diagnostic: %q", d.Reasons[0])
	}
	for _, step := range state.RoundSteps {
		if step.ImplementationVersion != d.Implementation {
			t.Fatal("step version mismatch")
		}
	}
}
