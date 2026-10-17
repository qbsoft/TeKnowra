package types

import "testing"

func TestAgentExecutionEngineExplicitAndFailClosed(t *testing.T) {
	for _, engine := range []string{"", "native", "langflow", "eino"} {
		if err := ValidateAgentExecutionEngine(engine); err != nil {
			t.Fatal(err)
		}
	}
	for _, engine := range []string{"https://external", "Langflow", "unknown"} {
		if ValidateAgentExecutionEngine(engine) == nil {
			t.Fatal("unsupported engine accepted")
		}
	}
	c := CustomAgentConfig{ExecutionEngine: "langflow", AgentMode: AgentModeSmartReasoning}
	if err := c.ValidateExecutionEngine(); err != nil {
		t.Fatal(err)
	}
	c.AgentMode = AgentModeQuickAnswer
	if c.ValidateExecutionEngine() == nil {
		t.Fatal("Langflow bypassed agent execution")
	}
}

func TestEinoPageSelectionRequiresPublishedDefinition(t *testing.T) {
	c := CustomAgentConfig{ExecutionEngine: "eino", AgentMode: AgentModeSmartReasoning}
	if c.ValidateExecutionEngine() == nil {
		t.Fatal("empty definition accepted")
	}
	c.EinoDefinition = "arbitrary-code"
	if c.ValidateExecutionEngine() == nil {
		t.Fatal("unpublished definition accepted")
	}
	c.EinoDefinition = "knowledge-summary-v1"
	off := false
	c.MemoryEnabled = &off
	c.KBSelectionMode = "selected"
	c.KnowledgeBases = []string{"kb"}
	c.MCPSelectionMode = "none"
	c.AllowedTools = []string{"knowledge_search"}
	if err := c.ValidateExecutionEngine(); err != nil {
		t.Fatal(err)
	}
	c.AgentMode = AgentModeQuickAnswer
	if c.ValidateExecutionEngine() == nil {
		t.Fatal("Eino bypassed agent execution")
	}
	c.ExecutionEngine = "native"
	if c.ValidateExecutionEngine() == nil {
		t.Fatal("stale Eino definition accepted for native engine")
	}
}
