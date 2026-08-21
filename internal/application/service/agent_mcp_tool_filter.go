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
// # Why an empty list means "allow everything"
//
// An agent whose AllowedTools is empty is running on DefaultAllowedTools(),
// which lists only built-ins. Treating that as "no MCP tools" would silently
// disarm every agent that has not been re-saved since this shipped. Empty means
// unconfigured, not empty-set.
func applyMCPToolAllowlist(
	ctx context.Context,
	registry *tools.ToolRegistry,
	config *types.AgentConfig,
) {
	if registry == nil || config == nil || len(config.AllowedTools) == 0 {
		return
	}

	// Only act once the agent actually names an MCP tool. Without this, an
	// agent configured before per-tool selection existed — its list holds
	// built-ins only — would lose every MCP tool the moment this shipped.
	allowed := make(map[string]struct{}, len(config.AllowedTools))
	namesMCP := false
	for _, name := range config.AllowedTools {
		allowed[name] = struct{}{}
		if strings.HasPrefix(name, mcpToolPrefix) {
			namesMCP = true
		}
	}
	if !namesMCP {
		return
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
