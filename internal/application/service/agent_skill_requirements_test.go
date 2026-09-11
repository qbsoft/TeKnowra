package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode"

	"github.com/Tencent/WeKnora/internal/agent/skills"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
)

// reqStubTool is the smallest thing the registry accepts; built-ins are in
// the registry at build time, unlike MCP tools since the catalog rework.
type reqStubTool struct{ name string }

func (s *reqStubTool) Name() string                { return s.name }
func (s *reqStubTool) Description() string         { return "" }
func (s *reqStubTool) Parameters() json.RawMessage { return json.RawMessage(`{}`) }
func (s *reqStubTool) Execute(context.Context, json.RawMessage) (*types.ToolResult, error) {
	return &types.ToolResult{Success: true}, nil
}

func registryOfBuiltins(names ...string) *tools.ToolRegistry {
	r := tools.NewToolRegistry()
	for _, n := range names {
		r.RegisterTool(&reqStubTool{name: n})
	}
	return r
}

func TestUnmetSkillRequirements_ReportsToolsTheAgentCannotCall(t *testing.T) {
	r := registryOfBuiltins("thinking")
	allowed := []string{
		"thinking",
		tools.AgentMCPToolKey(crmSvc, "list_review_templates"),
		tools.AgentMCPToolKey(mailSvc, "send_email"),
	}
	meta := []*skills.SkillMetadata{{
		Name:          "tyer-contract-review",
		RequiresTools: []string{"list_review_templates", "submit_review_finding"},
	}}

	unmet := unmetSkillRequirements(r, allowed, meta)

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

// A requirement is written the way the MCP server names the tool. The grant
// key carries that name as its last segment — the check must admit the bare
// name, not demand the full key.
func TestUnmetSkillRequirements_MatchesTheServerReportedName(t *testing.T) {
	r := registryOfBuiltins()
	allowed := []string{tools.AgentMCPToolKey(crmSvc, "get_review_summary")}

	unmet := unmetSkillRequirements(r, allowed, []*skills.SkillMetadata{
		{Name: "s", RequiresTools: []string{"get_review_summary"}},
	})

	if len(unmet) != 0 {
		t.Errorf("unmet = %v; the tool is granted and should count as callable", unmet)
	}
}

// The dangerous direction: reporting a requirement as satisfied when it is
// not. "email" must not match a grant of "send_email".
func TestUnmetSkillRequirements_DoesNotAcceptASuffixMatch(t *testing.T) {
	r := registryOfBuiltins()
	allowed := []string{tools.AgentMCPToolKey(mailSvc, "send_email")}

	unmet := unmetSkillRequirements(r, allowed, []*skills.SkillMetadata{
		{Name: "s", RequiresTools: []string{"email"}},
	})

	if len(unmet) != 1 {
		t.Fatalf("no tool named \"email\" is granted, but it was reported as available")
	}
}

func TestUnmetSkillRequirements_MatchesBuiltinToolsByExactName(t *testing.T) {
	r := registryOfBuiltins("thinking", "data_analysis")

	unmet := unmetSkillRequirements(r, []string{"thinking", "data_analysis"},
		[]*skills.SkillMetadata{
			{Name: "s", RequiresTools: []string{"data_analysis", "cronjob"}},
		})

	if len(unmet) != 1 || len(unmet[0].Missing) != 1 || unmet[0].Missing[0] != "cronjob" {
		t.Errorf("unmet = %v, want only cronjob missing", unmet)
	}
}

// An empty allowed_tools means the agent runs on defaults and reaches MCP
// tools through discovery, which this check cannot enumerate. Unverifiable
// must stay silent: a warning that fires for every legacy agent teaches
// people to ignore the warning.
func TestUnmetSkillRequirements_StaysSilentWhenItCannotVerify(t *testing.T) {
	r := registryOfBuiltins("thinking")

	unmet := unmetSkillRequirements(r, nil, []*skills.SkillMetadata{
		{Name: "s", RequiresTools: []string{"send_email"}},
	})

	if len(unmet) != 0 {
		t.Errorf("unmet = %v, want none: an unconfigured list is unverifiable, not missing", unmet)
	}
}

// Most skills predate the field. Declaring nothing is not a claim that the
// skill needs nothing, so it can never be unmet.
func TestUnmetSkillRequirements_IgnoresSkillsThatDeclareNothing(t *testing.T) {
	r := registryOfBuiltins("thinking")

	unmet := unmetSkillRequirements(r, []string{"thinking"},
		[]*skills.SkillMetadata{{Name: "old-skill"}})

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
	for _, r := range line {
		if r > unicode.MaxASCII {
			t.Errorf("log line contains non-ASCII %q; it will mojibake on GBK consoles", string(r))
		}
	}
}
