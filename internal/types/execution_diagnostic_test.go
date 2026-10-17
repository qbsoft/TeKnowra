package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExecutionDiagnosticPersistsInternallyButNeverInMessageJSON(t *testing.T) {
	d := NewExecutionDiagnostic("eino", "validate", "fact_audit_rejected", []string{"金额不符；Bearer secret-test-token; api_key=sk-test-private"})
	m := Message{ExecutionContext: MessageExecutionContext{ExecutionDiagnostics: []ExecutionDiagnostic{d}}}
	stored, err := m.ExecutionContext.Value()
	if err != nil {
		t.Fatal(err)
	}
	var restored MessageExecutionContext
	if err = restored.Scan(stored); err != nil {
		t.Fatal(err)
	}
	if len(restored.ExecutionDiagnostics) != 1 || !strings.Contains(restored.ExecutionDiagnostics[0].Reasons[0], "金额不符") {
		t.Fatal("diagnostic lost")
	}
	if strings.Contains(string(stored.([]byte)), "secret-test-token") || strings.Contains(string(stored.([]byte)), "sk-test-private") {
		t.Fatal("credentials persisted")
	}
	raw, _ := json.Marshal(m)
	if strings.Contains(string(raw), "fact_audit_rejected") || strings.Contains(string(raw), "金额不符") {
		t.Fatal("internal diagnostic leaked")
	}
}

func TestEinoSaveRejectsUnsupportedCapabilitiesAndIncompleteResources(t *testing.T) {
	off := false
	good := CustomAgentConfig{ExecutionEngine: "eino", EinoDefinition: "contract-review-v1", AgentMode: AgentModeSmartReasoning, MemoryEnabled: &off, KBSelectionMode: "selected", KnowledgeBases: []string{"kb"}, AllowedTools: []string{"knowledge_search"}, MCPSelectionMode: "none"}
	if err := good.ValidateExecutionEngine(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*CustomAgentConfig){
		func(c *CustomAgentConfig) { c.SkillsSelectionMode = "all" },
		func(c *CustomAgentConfig) { c.AllowSessionAttachments = true },
		func(c *CustomAgentConfig) { c.KnowledgeBases = nil },
		func(c *CustomAgentConfig) { c.HistoryTurns = 1 },
		func(c *CustomAgentConfig) { c.MultiTurnEnabled = true },
	} {
		c := good
		mutate(&c)
		if c.ValidateExecutionEngine() == nil {
			t.Fatal("unsupported/incomplete configuration accepted")
		}
	}
}

func TestEinoDefaultsDoNotReenableUnsupportedHistory(t *testing.T) {
	off := false
	a := CustomAgent{Config: CustomAgentConfig{ExecutionEngine: "eino", EinoDefinition: "contract-review-v1", AgentMode: AgentModeSmartReasoning, MemoryEnabled: &off, KBSelectionMode: "selected", KnowledgeBases: []string{"kb"}, AllowedTools: []string{"knowledge_search"}, MCPSelectionMode: "none"}}
	a.EnsureDefaults()
	if a.Config.MultiTurnEnabled || a.Config.HistoryTurns != 0 {
		t.Fatal("defaults re-enabled unsupported Eino history")
	}
	if err := a.Config.ValidateExecutionEngine(); err != nil {
		t.Fatal(err)
	}
}
