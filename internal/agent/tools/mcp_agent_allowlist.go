package tools

import "strings"

// This file is ours, not upstream's. It is the only entry point for per-agent
// MCP tool authorization; the two enforcement checks live in mcp_catalog.go
// (visibleTools and checkEnabled) because those are the choke points every
// discovery and execution path shares.
//
// # Why the key is "mcp:<service_id>:<tool_name>"
//
// An agent's allowed_tools used to name MCP tools by their registry name,
// "mcp_<service>_<tool>". That name is derived from the service's display
// name, which sanitises lossily (a Chinese-named service collapses to
// "mcp__<tool>") and, since the catalog rework, carries a schema hash that
// changes when the tool's definition changes. Neither property belongs in a
// stored authorization: renaming a service or an upstream schema tweak would
// silently revoke every grant.
//
// The service ID is a UUID that survives renames, and the tool name is the
// MCP server's own — exactly the pair the catalog itself keys tools by
// (servers[id] and policies[tool.mcpTool.Name]). ':' cannot appear in a UUID,
// so the first and last segment are unambiguous even if a tool name contains
// ':'.

// AgentMCPToolKeyPrefix distinguishes per-agent MCP grants from built-in tool
// names inside allowed_tools. Built-ins are bare names ("thinking"); the old
// registry-name format used "mcp_" with an underscore, so the colon keeps the
// two generations distinct as well.
const AgentMCPToolKeyPrefix = "mcp:"

// AgentMCPToolKey is the allowed_tools entry that grants one MCP tool.
func AgentMCPToolKey(serviceID, toolName string) string {
	return AgentMCPToolKeyPrefix + serviceID + ":" + toolName
}

// ParseAgentMCPToolKey splits a grant back into its service ID and tool name.
// ok is false for anything that is not an MCP grant, including the legacy
// "mcp_" registry-name form.
func ParseAgentMCPToolKey(key string) (serviceID, toolName string, ok bool) {
	rest, found := strings.CutPrefix(key, AgentMCPToolKeyPrefix)
	if !found {
		return "", "", false
	}
	serviceID, toolName, found = strings.Cut(rest, ":")
	if !found || serviceID == "" || toolName == "" {
		return "", "", false
	}
	return serviceID, toolName, true
}

// SetAgentMCPToolAllowlist narrows the registry's MCP catalog to the tools
// allow admits. A nil allow removes the restriction. Calling this on a
// registry that has no MCP catalog installed is a no-op, so the caller does
// not need to know whether any MCP service registered.
func (r *ToolRegistry) SetAgentMCPToolAllowlist(allow func(serviceID, toolName string) bool) {
	if c := r.mcpCatalog(); c != nil {
		c.agentAllow = allow
	}
}
