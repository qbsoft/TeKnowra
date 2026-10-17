package config

import "testing"

func TestLangflowBindingIsolation(t *testing.T) {
	c := &Config{}
	raw := []byte(`{"bindings":[{"agent_id":"lf","flow_id":"flow","workspace_id":1,"models":["m"],"nodes":[{"id":"model","kind":"model"}]}]}`)
	if err := c.loadLangflowBindings(raw); err != nil {
		t.Fatal(err)
	}
	b, ok := c.LangflowBinding("lf")
	if !ok {
		t.Fatal("missing binding")
	}
	b.Models[0] = "foreign"
	original, _ := c.LangflowBinding("lf")
	if original.Models[0] != "m" {
		t.Fatal("caller changed server binding")
	}
	c.einoDefinitions = map[string]string{"lf": "knowledge-summary-v1"}
	if err := c.loadLangflowBindings(raw); err == nil {
		t.Fatal("dual executor binding accepted")
	}
}
