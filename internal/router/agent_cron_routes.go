package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAgentCronRoutes mounts a user's own scheduled-task management.
//
// No RBAC guard beyond the group's authentication: every action is already
// scoped to the caller inside the service, and a job belongs to the person who
// created it rather than to the tenant. Adding a role check here would only
// stop people managing their own jobs.
func RegisterAgentCronRoutes(v1 *gin.RouterGroup, h *handler.AgentCronHandler) {
	jobs := v1.Group("/agent-cron/jobs")
	{
		jobs.GET("", h.ListJobs)
		jobs.DELETE("/:id", h.DeleteJob)
		jobs.POST("/:id/pause", h.PauseJob)
		jobs.POST("/:id/resume", h.ResumeJob)
		jobs.POST("/:id/run", h.RunJob)
	}
}
