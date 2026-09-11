package tools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The agent allowlist must bind both choke points: the discovery directory
// (visibleTools) and execution (checkEnabled). Filtering only the directory
// would leave a described or remembered tool_ref callable after the grant is
// gone; filtering only execution would advertise tools the agent cannot use.
func TestAgentAllowlistFiltersDiscovery(t *testing.T) {
	ctx, r, _, _, _ := catalogFixture(t, 3)
	r.SetAgentMCPToolAllowlist(func(serviceID, toolName string) bool {
		return serviceID == "server-1" && toolName == "tool_001"
	})

	page := discoverPage(ctx, t, r, map[string]any{"mode": "list_tools", "server_id": "server-1"})
	require.Equal(t, 1, page.Total, "only the granted tool may be listed")
	require.Equal(t, "tool_001", page.Tools[0].Name)

	// describe on an ungranted tool must not hand out a callable reference.
	raw, _ := json.Marshal(map[string]any{
		"mode": "describe", "server_id": "server-1", "tool_name": "tool_000",
	})
	result, err := r.ExecuteTool(ctx, ToolDiscoverMCPTools, raw)
	require.NoError(t, err)
	require.False(t, result.Success, "describe must refuse a tool outside the agent's grants")
}

func TestAgentAllowlistBlocksCallsEvenAfterDescribe(t *testing.T) {
	ctx, r, _, _, _ := catalogFixture(t, 3)
	described := describeTool(ctx, t, r, "server-1", "tool_001")

	// The grant is revoked after describe — the model still holds a valid
	// tool_ref. Execution is where the line must hold.
	r.SetAgentMCPToolAllowlist(func(string, string) bool { return false })
	raw, _ := json.Marshal(map[string]any{
		"tool_ref": described.ToolRef, "arguments": map[string]any{"count": 1},
	})
	result, err := r.ExecuteTool(ctx, ToolCallMCPTool, raw)
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Contains(t, result.Error, "allowed tools")
}

func TestAgentAllowlistNilMeansUnrestricted(t *testing.T) {
	ctx, r, _, _, _ := catalogFixture(t, 3)
	r.SetAgentMCPToolAllowlist(nil)
	page := discoverPage(ctx, t, r, map[string]any{"mode": "list_tools", "server_id": "server-1"})
	require.Equal(t, 3, page.Total, "nil predicate must not restrict anything")
}

// Setting an allowlist on a registry without an MCP catalog must be a no-op,
// not a panic: the caller cannot know whether any MCP service registered.
func TestAgentAllowlistWithoutCatalogIsANoop(t *testing.T) {
	r := NewToolRegistry()
	require.NotPanics(t, func() {
		r.SetAgentMCPToolAllowlist(func(string, string) bool { return false })
	})
}
