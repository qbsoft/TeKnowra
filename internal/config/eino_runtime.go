package config

import (
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/types"
	"io"
	"strings"
)

const ContractReviewDefinition = einodef.ContractReview
const CustomerCreditDefinition = einodef.CustomerCredit

// EinoDefinitionForAgent consumes only host-loaded Agent configuration. The
// runtime fields are json:"-"; a chat request cannot select a definition.
func (c *Config) EinoDefinitionForAgent(a *types.AgentConfig) (string, bool) {
	if a == nil {
		return "", false
	}
	if a.ExecutionEngine == "eino" {
		_, ok := einodef.Lookup(a.EinoDefinition)
		return a.EinoDefinition, ok
	}
	return c.EinoDefinition(a.AgentID)
}

// EinoDefinition is server-controlled. Chat requests cannot select an engine.
func (c *Config) EinoDefinition(agentID string) (string, bool) {
	if c == nil {
		return "", false
	}
	d, ok := c.einoDefinitions[agentID]
	return d, ok
}

func (c *Config) loadEinoDefinitions(raw string) error {
	c.einoDefinitions = nil
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	fail := func() error {
		return fmt.Errorf("TEKNOWRA_EINO_AGENTS must map distinct agent IDs to supported published definitions")
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return fail()
	}
	defs := map[string]string{}
	for dec.More() {
		tok, err = dec.Token()
		if err != nil {
			return fail()
		}
		id, ok := tok.(string)
		if !ok || strings.TrimSpace(id) != id || id == "" {
			return fail()
		}
		if _, dup := defs[id]; dup {
			return fail()
		}
		// Protect the existing deployed contract reviewer during the V2 pilot.
		if id == "854499ab-7e55-4be0-a681-807a096fea4e" {
			return fmt.Errorf("existing contract reviewer cannot be reassigned to the Eino pilot")
		}
		var d string
		if err = dec.Decode(&d); err != nil {
			return fail()
		}
		if _, published := einodef.Lookup(d); !published {
			return fail()
		}
		defs[id] = d
	}
	if _, err = dec.Token(); err != nil {
		return fail()
	}
	if _, err = dec.Token(); err != io.EOF {
		return fail()
	}
	c.einoDefinitions = defs
	return nil
}
