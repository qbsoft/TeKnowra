package service

import (
	"context"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// applyMCPToolAllowlist narrows an agent's MCP tools down to the ones its
// AllowedTools list grants.
//
// # Why this exists
//
// The platform grants built-in tools and MCP tools at different granularities:
// built-ins are picked one by one out of AllowedTools, while MCP tools arrive
// as whole services. So "let this agent send mail but not touch contract
// review" is not expressible when both live behind one service. This function
// is the missing granularity: the service list stays the source of candidates,
// and AllowedTools is the single answer to "what may this agent actually use".
//
// # How it enforces, after the catalog rework
//
// Upstream moved MCP from eager per-tool registration to a discovery catalog:
// the registry holds a directory tool and one call entry point, and concrete
// tools materialise on demand. There is nothing left to sweep out of the
// registry, so the old post-registration Unregister pass would silently allow
// everything. Instead this hands the catalog a predicate that visibleTools
// (discovery) and checkEnabled (every execution path, constructed tool_refs
// included) both consult — the same two choke points the tenant-wide
// enabled policy runs through.
//
// # The list is the whole answer
//
// A grant that is not in AllowedTools does not exist. Grants use the stable
// "mcp:<service_id>:<tool_name>" form (see tools.AgentMCPToolKey for why the
// registry name stopped being usable). A non-empty list containing no MCP
// grant therefore blocks every MCP tool — there is no "unconfigured" reading
// of a list that names only built-ins; we removed that compat rule after it
// made stored config lie about what agents could call.
//
// An empty AllowedTools still means unconfigured: such agents run on
// DefaultAllowedTools() and are not restricted here.
func applyMCPToolAllowlist(
	ctx context.Context,
	registry *tools.ToolRegistry,
	config *types.AgentConfig,
) {
	if registry == nil || config == nil {
		return
	}
	allow, legacy, restrict := agentMCPToolPredicate(config.AllowedTools)
	if !restrict {
		return
	}
	if len(legacy) > 0 {
		// A registry-name grant ("mcp_mail_send_email") predates the catalog
		// rework and cannot be resolved to a service ID anymore. It grants
		// nothing; saying so here is what turns "the agent lost its tools"
		// from a mystery into a config fix.
		logger.Warnf(ctx, "Ignoring %d legacy MCP grant(s) in allowed_tools "+
			"(re-save the agent's tool selection to convert them): %v",
			len(legacy), legacy)
	}
	registry.SetAgentMCPToolAllowlist(allow)
	logger.Infof(ctx, "Agent MCP allowlist active")
}

// agentMCPToolPredicate turns an allowed_tools list into the catalog
// predicate. restrict is false only for an empty list (unconfigured, running
// on defaults). legacy collects old registry-name grants, which admit nothing
// but deserve a log line.
func agentMCPToolPredicate(
	allowedTools []string,
) (allow func(serviceID, toolName string) bool, legacy []string, restrict bool) {
	if len(allowedTools) == 0 {
		return nil, nil, false
	}
	granted := make(map[string]struct{}, len(allowedTools))
	for _, name := range allowedTools {
		if _, _, ok := tools.ParseAgentMCPToolKey(name); ok {
			granted[name] = struct{}{}
			continue
		}
		if strings.HasPrefix(name, "mcp_") {
			legacy = append(legacy, name)
		}
	}
	return func(serviceID, toolName string) bool {
		_, ok := granted[tools.AgentMCPToolKey(serviceID, toolName)]
		return ok
	}, legacy, true
}
