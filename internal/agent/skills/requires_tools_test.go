package skills

import (
	"strings"
	"testing"
)

// A skill's body already names the tools it calls — tyer-contract-review walks
// the reader through list_review_templates, get_review_checklist and friends.
// requires_tools is that same list, moved somewhere a machine can read it, so
// enabling the skill on an agent that cannot call those tools is caught while
// configuring instead of halfway through a conversation.
func TestParseSkillFileReadsRequiresTools(t *testing.T) {
	content := `---
name: contract-review
description: 按清单审合同
requires_tools:
  - list_review_templates
  - get_review_checklist
---

正文。
`
	skill, err := ParseSkillFile(content)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	want := []string{"list_review_templates", "get_review_checklist"}
	if len(skill.RequiresTools) != len(want) {
		t.Fatalf("RequiresTools = %v, want %v", skill.RequiresTools, want)
	}
	for i, w := range want {
		if skill.RequiresTools[i] != w {
			t.Errorf("RequiresTools[%d] = %q, want %q", i, skill.RequiresTools[i], w)
		}
	}
}

// Every skill written before this field existed has no requires_tools. Those
// must keep loading, and must not be reported as "requires nothing but we are
// not sure" — absent means the author made no claim.
func TestSkillsWithoutRequiresToolsStillLoad(t *testing.T) {
	skill, err := ParseSkillFile("---\nname: plain\ndescription: 没有声明依赖\n---\n\n正文。\n")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if skill.RequiresTools != nil {
		t.Errorf("RequiresTools = %v, want nil for a skill that declares none", skill.RequiresTools)
	}
}

// The requirement is matched against tool names, so a value that can never be
// a tool name is a typo that would otherwise sit there matching nothing and
// warning forever.
func TestRequiresToolsRejectsUnusableNames(t *testing.T) {
	for _, bad := range []string{"has space", "<tag>", strings.Repeat("x", MaxToolNameLength+1)} {
		content := "---\nname: t\ndescription: d\nrequires_tools:\n  - \"" + bad + "\"\n---\n\n正文。\n"
		if _, err := ParseSkillFile(content); err == nil {
			t.Errorf("requires_tools accepted %q; it can never match a tool name", bad)
		}
	}
}

// Metadata is what the agent editor and the system prompt see. A requirement
// that stops at Level 1 cannot be checked before the skill is loaded, which is
// exactly when the check is useful.
func TestMetadataCarriesRequiresTools(t *testing.T) {
	skill := &Skill{Name: "t", Description: "d", RequiresTools: []string{"send_email"}}
	if got := skill.ToMetadata().RequiresTools; len(got) != 1 || got[0] != "send_email" {
		t.Errorf("metadata RequiresTools = %v, want [send_email]", got)
	}
}
