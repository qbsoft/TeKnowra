package langflowdef

import "testing"

const validBindings = `{"bindings":[{"agent_id":"agent-a","flow_id":"flow-a","workspace_id":1,"models":["model-a"],"knowledge_bases":["kb-a"],"nodes":[{"id":"model-node","kind":"model"},{"id":"kb-node","kind":"knowledge"}]}]}`

func TestBindingsSupportDifferentCapabilityCombinations(t *testing.T) {
	b, err := ParseBindings([]byte(validBindings))
	if err != nil || b["agent-a"].FlowID != "flow-a" {
		t.Fatalf("bindings: %v %v", b, err)
	}
	raw := `{"bindings":[{"agent_id":"agent-b","flow_id":"flow-b","workspace_id":2,"models":["model-b"],"tools":[{"service_id":"crm-b","name":"lookup"}],"nodes":[{"id":"m","kind":"model"},{"id":"tools","kind":"mcp"}]}]}`
	b, err = ParseBindings([]byte(raw))
	if err != nil || !b["agent-b"].AllowsTool("crm-b", "lookup") || b["agent-b"].AllowsTool("crm-a", "lookup") {
		t.Fatalf("service scope: %v", err)
	}
}

func TestBindingsRejectAmbiguousOrUnboundedConfiguration(t *testing.T) {
	for _, raw := range []string{
		validBindings + `{}`,
		`{"bindings":[],"api_key":"never"}`,
		`{"bindings":[{"agent_id":"a","flow_id":"f","workspace_id":1,"models":["m"],"nodes":[{"id":"n","kind":"model"},{"id":"n","kind":"knowledge"}]}]}`,
		`{"bindings":[{"agent_id":"a","flow_id":"f","workspace_id":1,"models":["m"],"nodes":[{"id":"n","kind":"mcp"}]}]}`,
		`{"bindings":[{"agent_id":"a","flow_id":"f","workspace_id":0,"models":["m"],"nodes":[{"id":"n","kind":"model"}]}]}`,
	} {
		if _, err := ParseBindings([]byte(raw)); err == nil {
			t.Fatalf("unsafe configuration accepted: %s", raw)
		}
	}
}

func TestBindingsRejectDuplicateAgents(t *testing.T) {
	raw := `{"bindings":[{"agent_id":"a","flow_id":"f","workspace_id":1,"models":["m"],"nodes":[{"id":"n","kind":"model"}]},{"agent_id":"a","flow_id":"g","workspace_id":1,"models":["m"],"nodes":[{"id":"n","kind":"model"}]}]}`
	if _, err := ParseBindings([]byte(raw)); err == nil {
		t.Fatal("duplicate agent accepted")
	}
}
