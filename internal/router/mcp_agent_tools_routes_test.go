package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// agent-tools returns an MCP service's tool names and descriptions — the same
// data as the sibling GET /mcp-services/:id/tools, only spelled the way an
// agent config spells it. Living in its own file to stay clear of upstream's
// routes_infra.go is a merge convenience, and the first version of that file
// let the convenience cost the route its guards: it was registered bare on v1,
// so it carried neither the Viewer+ role check nor the MCP capability that
// every sibling route carries.
//
// Nothing about the response made that visible, which is why it is pinned here
// rather than left to review.
func TestAgentToolsRouteCarriesTheSameGuardsAsItsSibling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")

	RegisterMCPAgentToolsRoutes(v1, &handler.MCPAgentToolsHandler{}, g)

	policy := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/mcp-services/:id/agent-tools")
	if !policy.RequireFullAccess {
		t.Error("an API key without full access can list a service's tools")
	}
	if !policyHasCapability(policy, types.APIKeyCapabilityManageMCPServices) {
		t.Errorf("policy capabilities = %#v, want manage_mcp_services", policy.Capabilities)
	}
}
