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
func RegisterMCPAgentToolsRoutes(v1 *gin.RouterGroup, h *handler.MCPAgentToolsHandler) {
	v1.GET("/mcp-services/:id/agent-tools", h.ListAgentTools)
}
