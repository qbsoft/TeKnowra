package langflowdef

import (
	"encoding/json"
	"testing"
)

func materialBinding() Binding {
	return Binding{AgentID: "a", FlowID: "f", WorkspaceID: 1, Models: []string{"m"}, AllowSessionAttachments: true, Skills: []string{"review"}, SandboxConfigID: "box", Nodes: []NodeBinding{{ID: "model", Kind: "model"}, {ID: "skill", Kind: "skill"}}}
}

const calculationSchema = `{"type":"object","properties":{"amount":{"type":"string","pattern":"^[0-9]{1,12}(\\.[0-9]{1,2})?$"},"percent":{"type":"number","minimum":0,"maximum":100}},"required":["amount","percent"],"additionalProperties":false}`

func TestMaterialSkillsAndStructuredScripts(t *testing.T) {
	b := materialBinding()
	if err := b.Validate(); err != nil {
		t.Fatal("read-only skills should compose with attachments:", err)
	}
	b.AllowSkillScripts = true
	if b.Validate() == nil {
		t.Fatal("attachment scripts without reviewed policy allowed")
	}
	b.StructuredScripts = []StructuredScript{{Skill: "review", Path: "scripts/calculate.py", InputSchema: json.RawMessage(calculationSchema)}}
	if err := b.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		skill, path, stdin string
		args               []string
		ok                 bool
	}{
		{"review", "scripts/calculate.py", `{"amount":"1000.50","percent":40}`, nil, true},
		{"review", "scripts/other.py", `{"amount":"1000","percent":40}`, nil, false},
		{"other", "scripts/calculate.py", `{"amount":"1000","percent":40}`, nil, false},
		{"review", "scripts/calculate.py", `{"amount":"1000","percent":40}`, []string{"/workspace/input/old.pdf"}, false},
		{"review", "scripts/calculate.py", `{"amount":"1000","percent":101}`, nil, false},
		{"review", "scripts/calculate.py", `{"amount":"1000","percent":40,"path":"/workspace/input/old.pdf"}`, nil, false},
		{"review", "scripts/calculate.py", `{"amount":"/workspace/input/old.pdf","percent":40}`, nil, false},
		{"review", "scripts/calculate.py", `{"percent":40}`, nil, false},
		{"review", "scripts/calculate.py", `{"amount":"1000","percent":40} {}`, nil, false},
	} {
		if err := b.ValidateScriptInput(tc.skill, tc.path, tc.args, tc.stdin); (err == nil) != tc.ok {
			t.Errorf("%s %s: expected allowed=%v, got %v", tc.path, tc.stdin, tc.ok, err)
		}
	}
	clone := b.Clone()
	clone.StructuredScripts[0].InputSchema[0] = 'x'
	if b.StructuredScripts[0].InputSchema[0] != '{' {
		t.Fatal("cloned publication aliases schema")
	}
}

func TestStructuredScriptsRejectInvalidPublication(t *testing.T) {
	for _, schema := range []string{`{"$ref":"https://example.com/schema"}`, `{"type":"object","properties":{"data":{"type":"object"}},"additionalProperties":false}`, `{"type":"object","properties":{"amount":{"$ref":"file:///secret"}},"additionalProperties":false}`, `{"type":"object","properties":{"amount":{"type":"string"}}}`} {
		b := materialBinding()
		b.AllowSkillScripts = true
		b.StructuredScripts = []StructuredScript{{Skill: "review", Path: "scripts/calculate.py", InputSchema: json.RawMessage(schema)}}
		if b.Validate() == nil {
			t.Error("unsafe/unbounded schema accepted", schema)
		}
	}
	b := materialBinding()
	b.AllowSessionAttachments = false
	b.AllowSkillScripts = true
	if err := b.Validate(); err != nil {
		t.Fatal("legacy script publications must remain compatible", err)
	}
}
