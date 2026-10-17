package service

import (
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestLangflowSkillsRequireHostGrantAndMatchingSandbox(t *testing.T) {
	no := false
	b := langflowdef.Binding{Skills: []string{"tyer-demo"}, SandboxConfigID: "sandbox"}
	c := &types.AgentConfig{SkillsEnabled: true, AllowedSkills: []string{"tyer-demo"}, SandboxConfigID: "sandbox", MemoryEnabled: &no}
	if err := validateLangflowCapabilities(b, c); err != nil {
		t.Fatal(err)
	}
	c.AllowedSkills = []string{"foreign"}
	if validateLangflowCapabilities(b, c) == nil {
		t.Fatal("unselected skill allowed")
	}
}
