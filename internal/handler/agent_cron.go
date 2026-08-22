package handler

import (
	"net/http"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// AgentCronHandler exposes a user's own scheduled agent tasks.
//
// The tool is the primary way these get created — a user says "every morning
// at 9…" and the agent does it. This exists for the other half: seeing what is
// scheduled and stopping it, which nobody should have to hold a conversation
// to do.
//
// Every action is scoped to the caller by the service layer, so there is no
// tenant-admin view here: a job belongs to the person who created it.
type AgentCronHandler struct {
	cron interfaces.AgentCronManager
}

// NewAgentCronHandler constructs the handler.
func NewAgentCronHandler(cron interfaces.AgentCronManager) *AgentCronHandler {
	return &AgentCronHandler{cron: cron}
}

type agentCronListResponse struct {
	Success bool                  `json:"success"`
	Data    []*types.AgentCronJob `json:"data"`
}

type agentCronItemResponse struct {
	Success bool                `json:"success"`
	Data    *types.AgentCronJob `json:"data"`
}

// disabled reports whether the feature is switched off, and answers the
// request if so.
//
// The container injects a nil manager when WEKNORA_AGENT_CRON_ENABLED is not
// set. Returning 404 rather than 500 is deliberate: to a caller, a feature
// that was never turned on is indistinguishable from one that does not exist,
// and an error page would suggest something is broken.
func (h *AgentCronHandler) disabled(c *gin.Context) bool {
	if h.cron != nil {
		return false
	}
	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"error":   "定时任务功能未启用",
	})
	return true
}

// fail maps a service error onto a response.
//
// The service writes its errors for the end user (quota messages, schedule
// complaints), so they are passed through verbatim rather than replaced with a
// generic string.
func (h *AgentCronHandler) fail(c *gin.Context, status int, err error) {
	logger.Warnf(c.Request.Context(), "[AgentCron] request failed: %v", err)
	c.JSON(status, gin.H{"success": false, "error": err.Error()})
}

// ListJobs godoc
// @Summary      列出我的定时任务
// @Description  返回当前用户自己创建的定时任务，含上次执行结果
// @Tags         定时任务
// @Produce      json
// @Success      200 {object} agentCronListResponse
// @Router       /agent-cron/jobs [get]
func (h *AgentCronHandler) ListJobs(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	jobs, err := h.cron.List(c.Request.Context())
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err)
		return
	}
	if jobs == nil {
		jobs = []*types.AgentCronJob{}
	}
	c.JSON(http.StatusOK, agentCronListResponse{Success: true, Data: jobs})
}

// PauseJob godoc
// @Summary      暂停定时任务
// @Tags         定时任务
// @Produce      json
// @Param        id path string true "任务 ID"
// @Success      200 {object} agentCronItemResponse
// @Router       /agent-cron/jobs/{id}/pause [post]
func (h *AgentCronHandler) PauseJob(c *gin.Context) {
	h.setPaused(c, true)
}

// ResumeJob godoc
// @Summary      恢复定时任务
// @Tags         定时任务
// @Produce      json
// @Param        id path string true "任务 ID"
// @Success      200 {object} agentCronItemResponse
// @Router       /agent-cron/jobs/{id}/resume [post]
func (h *AgentCronHandler) ResumeJob(c *gin.Context) {
	h.setPaused(c, false)
}

func (h *AgentCronHandler) setPaused(c *gin.Context, paused bool) {
	if h.disabled(c) {
		return
	}
	job, err := h.cron.SetPaused(c.Request.Context(), c.Param("id"), paused)
	if err != nil {
		// The service returns the same "not found" for a job that belongs to
		// somebody else, so 404 is the honest status either way.
		h.fail(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, agentCronItemResponse{Success: true, Data: job})
}

// RunJob godoc
// @Summary      立即执行一次
// @Description  在后台跑一次，不影响原有排程
// @Tags         定时任务
// @Produce      json
// @Param        id path string true "任务 ID"
// @Success      202 {object} agentCronItemResponse
// @Router       /agent-cron/jobs/{id}/run [post]
func (h *AgentCronHandler) RunJob(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	job, err := h.cron.RunNow(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.fail(c, http.StatusConflict, err)
		return
	}
	// 202: the run was accepted onto the queue, not completed.
	c.JSON(http.StatusAccepted, agentCronItemResponse{Success: true, Data: job})
}

// DeleteJob godoc
// @Summary      删除定时任务
// @Tags         定时任务
// @Produce      json
// @Param        id path string true "任务 ID"
// @Success      200 {object} agentCronItemResponse
// @Router       /agent-cron/jobs/{id} [delete]
func (h *AgentCronHandler) DeleteJob(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	job, err := h.cron.Remove(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.fail(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, agentCronItemResponse{Success: true, Data: job})
}
