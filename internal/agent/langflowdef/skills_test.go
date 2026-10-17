package langflowdef

import "testing"

func TestSkillBindingsRequireExplicitSandboxAndCannotAlias(t *testing.T) {
	raw := []byte(`{"bindings":[{"agent_id":"a","flow_id":"f","workspace_id":1,"models":["m"],"skills":["tyer-selftest"],"sandbox_config_id":"sandbox","allow_skill_scripts":true,"nodes":[{"id":"m","kind":"model"},{"id":"s","kind":"skill"}]}]}`)
	parsed, err := ParseBindings(raw)
	if err != nil {
		t.Fatal(err)
	}
	b := parsed["a"]
	copy := b.Clone()
	copy.Skills[0] = "foreign"
	if b.Skills[0] != "tyer-selftest" || !b.AllowsSkill("tyer-selftest") || b.AllowsSkill("foreign") {
		t.Fatal("skill binding scope lost")
	}
	b.SandboxConfigID = ""
	if b.Validate() == nil {
		t.Fatal("unscoped skill sandbox accepted")
	}
	b.SandboxConfigID = "sandbox"
	b.Nodes = b.Nodes[:1]
	if b.Validate() == nil {
		t.Fatal("skill resources without a skill node accepted")
	}
	b.Nodes = append(b.Nodes, NodeBinding{ID: "s", Kind: "skill"})
	b.Skills = []string{"../foreign"}
	if b.Validate() == nil {
		t.Fatal("skill traversal accepted")
	}
	b.Skills = []string{"tyer-selftest", "tyer-selftest"}
	if b.Validate() == nil {
		t.Fatal("duplicate skill accepted")
	}
	b.Skills = nil
	if b.Validate() == nil {
		t.Fatal("skill node without skill accepted")
	}
}
