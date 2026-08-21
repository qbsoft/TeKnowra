package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterMCPAgentToolsRoutes mounts the lookup an agent author needs when
// filling in allowed_tools: which tools a service offers, under the names the
// registry gives them.
//
// Its own file and its own registration call so the addition stays out of
// routes_infra.go, where upstream keeps editing the MCP group.
//
// # Guards
//
// This route returns the same data as GET /mcp-services/:id/tools, only under
// different names, so it carries that route's protections rather than weaker
// ones: Viewer+ on the role axis, and the MCP-services capability on the API
// key axis. Keeping the file separate must not mean keeping the guards
// separate — the first version of this file registered the route bare on v1,
// which skipped both. A second group over the same prefix is how a route in
// its own file inherits them; gin allows it as long as the ":id" parameter is
// spelled the way the other group spells it.
func RegisterMCPAgentToolsRoutes(v1 *gin.RouterGroup, h *handler.MCPAgentToolsHandler, g *rbacGuards) {
	mcpServices := g.apiKeyGroup(
		v1.Group("/mcp-services"),
		apiKeyManageMCPServices(apiKeyFullAccess()),
	)
	mcpServices.GET("/:id/agent-tools", g.Viewer(), h.ListAgentTools)
}
