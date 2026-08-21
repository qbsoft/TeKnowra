package service

import (
	"context"
	"strings"
	"testing"
	"unicode"

	"github.com/Tencent/WeKnora/internal/agent/skills"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
)

// registryWithMCP builds a registry holding real MCPTools, so the names in it
// are produced by the same code that produces them at run time.
func registryWithMCP(pairs ...[2]string) *tools.ToolRegistry {
	r := tools.NewToolRegistry()
	for _, p := range pairs {
		r.RegisterTool(tools.NewMCPTool(
			&types.MCPService{ID: p[0], Name: p[0]},
			&types.MCPTool{Name: p[1]}, nil, nil, 0,
		))
	}
	return r
}

func TestUnmetSkillRequirements_ReportsToolsTheAgentCannotCall(t *testing.T) {
	r := registryWithMCP(
		[2]string{"CRM-REVIEW-MCP", "list_review_templates"},
		[2]string{"mail", "send_email"},
	)
	meta := []*skills.SkillMetadata{{
		Name:          "tyer-contract-review",
		RequiresTools: []string{"list_review_templates", "submit_review_finding"},
	}}

	unmet := unmetSkillRequirements(r, meta)

	if len(unmet) != 1 {
		t.Fatalf("unmet = %v, want one entry", unmet)
	}
	if got := unmet[0].Skill; got != "tyer-contract-review" {
		t.Errorf("skill = %q, want tyer-contract-review", got)
	}
	if len(unmet[0].Missing) != 1 || unmet[0].Missing[0] != "submit_review_finding" {
		t.Errorf("missing = %v, want [submit_review_finding]", unmet[0].Missing)
	}
}

// A requirement is written the way the MCP server names the tool. Matching it
// against the registry name — which carries a workspace-specific service
// prefix — would report every MCP requirement as missing.
func TestUnmetSkillRequirements_MatchesTheServerReportedName(t *testing.T) {
	r := registryWithMCP([2]string{"CRM-REVIEW-MCP", "get_review_summary"})

	unmet := unmetSkillRequirements(r, []*skills.SkillMetadata{
		{Name: "s", RequiresTools: []string{"get_review_summary"}},
	})

	if len(unmet) != 0 {
		t.Errorf("unmet = %v; the tool is registered as %v and should count as granted",
			unmet, r.ListTools())
	}
}

// The dangerous direction: reporting a requirement as satisfied when it is
// not. Suffix matching against registry names does exactly this — "email" is a
// suffix of "mcp_mail_send_email" — and the warning would then stay silent for
// the case it exists to catch.
func TestUnmetSkillRequirements_DoesNotAcceptASuffixMatch(t *testing.T) {
	r := registryWithMCP([2]string{"mail", "send_email"})

	unmet := unmetSkillRequirements(r, []*skills.SkillMetadata{
		{Name: "s", RequiresTools: []string{"email"}},
	})

	if len(unmet) != 1 {
		t.Fatalf("no tool named \"email\" is registered (%v), but it was reported as granted",
			r.ListTools())
	}
}

func TestUnmetSkillRequirements_MatchesBuiltinToolsByExactName(t *testing.T) {
	r := registryWith("thinking", "data_analysis")

	unmet := unmetSkillRequirements(r, []*skills.SkillMetadata{
		{Name: "s", RequiresTools: []string{"data_analysis", "cronjob"}},
	})

	if len(unmet) != 1 || len(unmet[0].Missing) != 1 || unmet[0].Missing[0] != "cronjob" {
		t.Errorf("unmet = %v, want only cronjob missing", unmet)
	}
}

// Most skills predate the field. Declaring nothing is not a claim that the
// skill needs nothing, so it can never be unmet.
func TestUnmetSkillRequirements_IgnoresSkillsThatDeclareNothing(t *testing.T) {
	r := registryWith("thinking")

	unmet := unmetSkillRequirements(r, []*skills.SkillMetadata{{Name: "old-skill"}})

	if len(unmet) != 0 {
		t.Errorf("unmet = %v, want none for a skill with no declaration", unmet)
	}
}

func TestLogUnmetSkillRequirements_NamesTheSkillAndTheTools(t *testing.T) {
	// The log line is the whole product here: it is what someone reads when a
	// skill quietly does nothing. Assert it carries both halves.
	line := formatUnmetRequirement(skillRequirementGap{
		Skill: "tyer-contract-review", Missing: []string{"submit_review_finding", "get_review_checklist"},
	})
	for _, want := range []string{"tyer-contract-review", "submit_review_finding", "get_review_checklist"} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q does not mention %q", line, want)
		}
	}
}

// Log lines travel through pipelines that do not all agree on encoding. A
// Windows console at the GBK codepage turned an em dash in this message into
// mojibake, which is how the character got noticed at all.
func TestUnmetRequirementLineIsASCII(t *testing.T) {
	line := formatUnmetRequirement(skillRequirementGap{
		Skill: "s", Missing: []string{"t"},
	})
	for i, r := range line {
		if r > unicode.MaxASCII {
			t.Errorf("log line holds %q at byte %d; keep it ASCII so it survives "+
				"a non-UTF-8 console: %q", r, i, line)
		}
	}
}

func TestUnmetSkillRequirements_SurvivesNilInputs(t *testing.T) {
	if got := unmetSkillRequirements(nil, nil); got != nil {
		t.Errorf("unmet = %v, want nil", got)
	}
	_ = context.Background()
}
