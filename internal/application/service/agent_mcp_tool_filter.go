package service

import (
	"context"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// mcpToolPrefix is what MCPTool.Name() puts in front of every MCP tool name.
//
// Declared here rather than exported from the tools package so this change
// touches no upstream file. The cost of that choice is drift: if upstream ever
// renames the prefix, this filter would quietly stop matching and every agent
// would silently regain the tools it was configured to lose. Silent is the
// dangerous part, so TestMCPToolPrefixMatchesReality pins the value against a
// real MCPTool name and fails loudly if it moves.
const mcpToolPrefix = "mcp_"

// applyMCPToolAllowlist narrows an agent's MCP tools down to the ones its
// AllowedTools list names.
//
// # Why this exists
//
// The platform grants built-in tools and MCP tools at different granularities:
// built-ins are picked one by one out of AllowedTools, while MCP tools arrive
// as whole services — select a service and every tool it exposes lands in the
// registry. So "let this agent send mail but not touch contract review" is not
// expressible when both live behind one service, and "take one tool from
// service A and one from service B" is not expressible at all.
//
// Claude Code separates the two concerns: `mcpServers` decides what to connect
// to, `tools` decides what may be called. This function is that separation —
// the service list stays the source of candidates, and AllowedTools becomes the
// single answer to "what may this agent actually use".
//
// # Why it filters after registration rather than during
//
// RegisterMCPTools is upstream code with a single call site. Threading an
// allowlist through its signature would put our change in the middle of a
// function upstream keeps editing. Removing entries afterwards keeps the whole
// decision here, in a file upstream does not have, and costs one line at the
// call site.
//
// The tools are never reachable by the model either way: registration and this
// sweep both happen while the engine is being built, before the first LLM call.
//
// # The list is the whole answer
//
// A name that is not in AllowedTools is not available, full stop. There is no
// "unconfigured" mode where an unlisted tool is reachable anyway.
//
// An earlier version had one: a list naming no MCP tool at all was read as
// "written before per-tool selection existed" and skipped the sweep, so every
// tool of every selected service stayed reachable. It existed only to keep
// already-deployed agents working, and it cost more than it saved — the editor
// showed six unticked checkboxes while the agent happily called all six, and
// the effective-tools preview had to grow a matching special case to stop
// lying. Config that does not mean what it says is worse than config that
// disarms an agent loudly.
//
// An empty AllowedTools still returns early, but for a different reason: an
// empty list is DefaultAllowedTools() territory, and the built-in defaults are
// resolved before this runs.
func applyMCPToolAllowlist(
	ctx context.Context,
	registry *tools.ToolRegistry,
	config *types.AgentConfig,
) {
	if registry == nil || config == nil || len(config.AllowedTools) == 0 {
		return
	}

	allowed := make(map[string]struct{}, len(config.AllowedTools))
	for _, name := range config.AllowedTools {
		allowed[name] = struct{}{}
	}

	var dropped []string
	for _, name := range registry.ListTools() {
		if !strings.HasPrefix(name, mcpToolPrefix) {
			continue // built-ins were already filtered when they were registered
		}
		if _, ok := allowed[name]; ok {
			continue
		}
		registry.Unregister(name)
		dropped = append(dropped, name)
	}

	if len(dropped) > 0 {
		// Info, not Warn: dropping tools is the configured outcome, and this
		// line is what explains "why can't the agent see that tool" later.
		logger.Infof(ctx, "Dropped %d MCP tool(s) not in the agent's allowed list: %v",
			len(dropped), dropped)
	}
}
