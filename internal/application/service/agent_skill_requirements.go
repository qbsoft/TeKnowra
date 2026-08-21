package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/skills"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/logger"
)

// skillRequirementGap is one skill and the tools it declared but did not get.
type skillRequirementGap struct {
	Skill   string
	Missing []string
}

// unmetSkillRequirements reports, for each enabled skill, the tools it declares
// in requires_tools that this agent cannot actually call.
//
// # Why this exists
//
// A skill's instructions name the tools they call — tyer-contract-review walks
// the model through list_review_templates, get_review_checklist and two more.
// Enabling that skill on an agent that was not granted those tools produces the
// worst kind of failure: the model reads instructions telling it to call
// something that is not there, improvises, and answers anyway. Nothing errors.
//
// The agent editor warns while the agent is being configured. This runs at
// build time and covers what the editor cannot: a service renamed, a tool
// unticked, or a skill updated to need something new, all long after anyone
// last opened the editor.
//
// # Why it only warns
//
// requires_tools is the skill author's claim, and the claim can be wrong in
// either direction — a skill may list a tool it no longer uses, or use one it
// forgot to list. Refusing to start an agent over an author's stale metadata
// would trade a degraded answer for no answer at all. So this logs and the
// agent runs.
//
// # Why matching is exact rather than by suffix
//
// A requirement names the tool the way its MCP server does ("send_email"). The
// registry name adds a prefix built from the workspace's name for the service
// ("mcp_mail_send_email"), which no portable skill file can predict. The
// tempting shortcut — does any registry name end with "_" + requirement — is
// wrong in the dangerous direction: "email" is a suffix of
// "mcp_mail_send_email", so a genuinely missing tool would be reported as
// granted and this whole function would go quiet exactly when it matters.
// Asking each MCP tool for its own source name avoids the guess.
func unmetSkillRequirements(
	registry *tools.ToolRegistry,
	metadata []*skills.SkillMetadata,
) []skillRequirementGap {
	if registry == nil || len(metadata) == 0 {
		return nil
	}

	// Every name this agent can call, in both spellings a requirement might
	// use: the registry name (built-ins are required under it) and, for MCP
	// tools, the name their server reports.
	callable := make(map[string]struct{})
	for _, name := range registry.ListTools() {
		callable[name] = struct{}{}
		tool, err := registry.GetTool(name)
		if err != nil {
			continue
		}
		if mcpTool, ok := tool.(*tools.MCPTool); ok {
			if source := mcpTool.SourceToolName(); source != "" {
				callable[source] = struct{}{}
			}
		}
	}

	var gaps []skillRequirementGap
	for _, meta := range metadata {
		if meta == nil || len(meta.RequiresTools) == 0 {
			continue // declared nothing; that is not a claim of needing nothing
		}
		var missing []string
		for _, required := range meta.RequiresTools {
			if _, ok := callable[required]; !ok {
				missing = append(missing, required)
			}
		}
		if len(missing) > 0 {
			gaps = append(gaps, skillRequirementGap{Skill: meta.Name, Missing: missing})
		}
	}
	return gaps
}

// formatUnmetRequirement renders one gap. Kept separate so a test can pin what
// the line actually says: this text is the entire product of the check, and a
// line that names the skill but not the tools sends the reader nowhere.
func formatUnmetRequirement(gap skillRequirementGap) string {
	return fmt.Sprintf(
		"skill %q declares requires_tools that this agent cannot call: %s "+
			"— the skill will run and improvise around the missing tools rather than fail",
		gap.Skill, strings.Join(gap.Missing, ", "))
}

// logUnmetSkillRequirements is the call-site wrapper: one line per skill, so a
// log search for the skill name finds it.
func logUnmetSkillRequirements(
	ctx context.Context,
	registry *tools.ToolRegistry,
	metadata []*skills.SkillMetadata,
) {
	for _, gap := range unmetSkillRequirements(registry, metadata) {
		logger.Warnf(ctx, "%s", formatUnmetRequirement(gap))
	}
}
