package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// The point of the field is the skill that actually uses it. Parsing the real
// file catches a frontmatter mistake that a hand-written fixture never would.
func TestPreloadedContractReviewDeclaresItsTools(t *testing.T) {
	path := filepath.Join("..", "..", "..", "skills", "preloaded", "tyer-contract-review", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("skill not present in this build: %v", err)
	}
	skill, err := ParseSkillFile(string(content))
	if err != nil {
		t.Fatalf("the shipped skill no longer parses: %v", err)
	}
	want := map[string]bool{
		"list_review_templates": false, "get_review_checklist": false,
		"submit_review_finding": false, "get_review_summary": false,
	}
	for _, got := range skill.RequiresTools {
		if _, ok := want[got]; !ok {
			t.Errorf("declares %q, which is not a tool this skill calls", got)
			continue
		}
		want[got] = true
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("the skill body calls %s but requires_tools does not declare it", name)
		}
	}
}
