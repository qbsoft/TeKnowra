package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterLangflowRoutes(r *gin.RouterGroup, h *handler.LangflowHandler, g *rbacGuards) {
	// Human-only metadata connection; API-key callers stay default-deny.
	r.GET("/agents/:id/langflow", g.OwnedAgentOrAdmin(), h.Info)
	r.GET("/agents/:id/langflow/draft-options", g.OwnedAgentOrAdmin(), h.DraftOptions)
	r.POST("/agents/:id/langflow/draft", g.OwnedAgentOrAdmin(), h.SetupDraft)
	r.POST("/agents/:id/langflow/design-session", g.OwnedAgentOrAdmin(), h.Create)
	r.GET("/agents/:id/langflow/releases", g.OwnedAgentOrAdmin(), h.ListReleases)
	r.POST("/agents/:id/langflow/releases", g.OwnedAgentOrAdmin(), h.Publish)
	r.POST("/agents/:id/langflow/releases/:release_id/activate", g.OwnedAgentOrAdmin(), h.Activate)
}
