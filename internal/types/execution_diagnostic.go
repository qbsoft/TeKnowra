package types

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

// ExecutionDiagnostic is internal task metadata, never an ordinary chat answer.
// Store classifications and bounded review issues, not prompts or drafts.
type ExecutionDiagnostic struct {
	Engine         string    `json:"engine"`
	Node           string    `json:"node"`
	Code           string    `json:"code"`
	Reasons        []string  `json:"reasons,omitempty"`
	Definition     string    `json:"definition,omitempty"`
	Implementation string    `json:"implementation,omitempty"`
	RunID          string    `json:"run_id,omitempty"`
	RecordedAt     time.Time `json:"recorded_at"`
}

var diagnosticSecret = regexp.MustCompile(`(?i)(bearer\s+[^\s;,"']+|sk-[a-z0-9_-]+|(?:api[_-]?key|access[_-]?token|refresh[_-]?token|client[_-]?secret|authorization|password|secret|token)["']?\s*[=:]\s*["']?[^\s;,"']+|[a-f0-9]{64}|eyJ[a-z0-9_-]{8,}\.[a-z0-9_-]+\.[a-z0-9_-]+)`)

func NewExecutionDiagnostic(engine, node, code string, reasons []string) ExecutionDiagnostic {
	d := ExecutionDiagnostic{Engine: engine, Node: node, Code: code, RecordedAt: time.Now()}
	for _, reason := range reasons {
		if len(d.Reasons) >= 8 {
			break
		}
		clean := diagnosticSecret.ReplaceAllString(reason, "[redacted]")
		chars := []rune(strings.TrimSpace(clean))
		if len(chars) > 512 {
			chars = chars[:512]
		}
		if len(chars) > 0 {
			d.Reasons = append(d.Reasons, string(chars))
		}
	}
	return d
}

// AuditDiagnostic observes a review response; it does not decide whether a
// result may publish. Existing independent review gates remain authoritative.
func AuditDiagnostic(engine, node, raw string) *ExecutionDiagnostic {
	var audit struct {
		Approved *bool     `json:"approved"`
		Issues   *[]string `json:"issues"`
	}
	if json.Unmarshal([]byte(raw), &audit) != nil || audit.Approved == nil || audit.Issues == nil {
		d := NewExecutionDiagnostic(engine, node, "fact_audit_invalid", nil)
		return &d
	}
	if !*audit.Approved || len(*audit.Issues) > 0 {
		d := NewExecutionDiagnostic(engine, node, "fact_audit_rejected", *audit.Issues)
		return &d
	}
	return nil
}

func ExecutionFailureCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "task_timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "task_cancelled"
	}
	if err == nil {
		return "engine_execution_failed"
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "finish=length"):
		return "model_output_truncated"
	case strings.Contains(text, "budget"):
		return "call_budget_exceeded"
	case strings.Contains(text, "授权"), strings.Contains(text, "权限"), strings.Contains(text, "permission"), strings.Contains(text, "authorization"):
		return "authorization_check_failed"
	case strings.Contains(text, "model stream"), strings.Contains(text, "provider"):
		return "model_provider_failed"
	default:
		return "engine_execution_failed"
	}
}
