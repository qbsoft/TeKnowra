package skills

import (
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

// DiscoverSkills is what tells the requirement check which tools a skill
// needs. It runs on every turn, so it reads the row rather than the archive —
// and that is exactly why it is easy to break: someone trimming the row read
// would not see any test go red without this one.
func TestTenantSourceExposesRequiresTools(t *testing.T) {
	declared := []string{"list_review_templates", "get_review_checklist"}
	encoded, err := json.Marshal(declared)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	src := NewTenantSkillSource([]*types.TenantSkillEntity{{
		Name:          "contract-review",
		Description:   "按清单审合同",
		RequiresTools: types.JSON(encoded),
		// usableSkillRow hides anything not ready-and-enabled; a row without
		// these is invisible and the test would pass vacuously.
		Enabled: true,
		Status:  types.SkillStatusReady,
	}}, nil)

	metadata, err := src.DiscoverSkills()
	if err != nil {
		t.Fatalf("DiscoverSkills: %v", err)
	}
	if len(metadata) != 1 {
		t.Fatalf("metadata = %v, want one skill", metadata)
	}
	got := metadata[0].RequiresTools
	if len(got) != len(declared) {
		t.Fatalf("RequiresTools = %v, want %v — the declaration was dropped "+
			"between the row and the check, and nothing else would report it",
			got, declared)
	}
	for i, w := range declared {
		if got[i] != w {
			t.Errorf("RequiresTools[%d] = %q, want %q", i, got[i], w)
		}
	}
}

// Most skills declare nothing. That must read as "no claim", never as an
// error, or every such skill would break discovery for the whole run.
func TestTenantSourceToleratesNoDeclaration(t *testing.T) {
	for name, raw := range map[string]types.JSON{
		"empty column": nil,
		"empty array":  types.JSON("[]"),
		"malformed":    types.JSON("{not json"),
	} {
		src := NewTenantSkillSource([]*types.TenantSkillEntity{{
			Name: "plain", RequiresTools: raw,
			Enabled: true, Status: types.SkillStatusReady,
		}}, nil)
		metadata, err := src.DiscoverSkills()
		if err != nil {
			t.Errorf("%s: DiscoverSkills failed: %v", name, err)
			continue
		}
		if len(metadata) != 1 || len(metadata[0].RequiresTools) != 0 {
			t.Errorf("%s: RequiresTools = %v, want empty", name, metadata[0].RequiresTools)
		}
	}
}
