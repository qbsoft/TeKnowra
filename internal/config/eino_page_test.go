package config

import (
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestSavedEinoDefinitionAndLegacyBindingRemainDistinct(t *testing.T) {
	c := &Config{}
	if err := c.loadEinoDefinitions(`{"legacy":"contract-review-v1"}`); err != nil {
		t.Fatal(err)
	}
	for _, a := range []*types.AgentConfig{{AgentID: "new", ExecutionEngine: "eino", EinoDefinition: "knowledge-summary-v1"}, {AgentID: "legacy"}} {
		if _, ok := c.EinoDefinitionForAgent(a); !ok {
			t.Fatal("valid saved or legacy definition rejected")
		}
	}
	if _, ok := c.EinoDefinitionForAgent(&types.AgentConfig{ExecutionEngine: "eino", EinoDefinition: "arbitrary-code"}); ok {
		t.Fatal("unpublished code accepted")
	}
	var injected types.AgentConfig
	if err := json.Unmarshal([]byte(`{"execution_engine":"eino","eino_definition":"knowledge-summary-v1"}`), &injected); err != nil {
		t.Fatal(err)
	}
	if injected.ExecutionEngine != "" || injected.EinoDefinition != "" {
		t.Fatal("chat JSON selected an execution engine")
	}
}
