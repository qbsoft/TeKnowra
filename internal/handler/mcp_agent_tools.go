package handler

import (
	"net/http"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// MCPAgentToolsHandler lists an MCP service's tools under the names an agent
// must use to allow them.
//
// The existing /mcp-services/:id/tools endpoint returns what the MCP server
// itself reports — "send_email". What goes into an agent's allowed_tools is the
// name the registry gives it — "mcp_mail_send_email". Without somewhere to read
// the second form, configuring per-tool access means guessing at a
// transformation or reading it out of the backend log.
//
// Deriving it here rather than in the browser keeps one implementation of the
// rule. The transformation drops every non-ASCII character, so a service named
// in Chinese yields "mcp__send_email" — surprising enough that seeing the real
// name is worth an endpoint on its own.
type MCPAgentToolsHandler struct {
	mcpServiceService interfaces.MCPServiceService
}

// NewMCPAgentToolsHandler constructs the handler.
func NewMCPAgentToolsHandler(svc interfaces.MCPServiceService) *MCPAgentToolsHandler {
	return &MCPAgentToolsHandler{mcpServiceService: svc}
}

// agentToolView pairs what the MCP server calls a tool with what an agent
// config has to call it.
type agentToolView struct {
	// ToolName is the name the MCP server reports.
	ToolName string `json:"tool_name"`
	// RegistryName is what goes into an agent's allowed_tools.
	RegistryName string `json:"registry_name"`
	Description  string `json:"description,omitempty"`
	// NameDegraded flags a service whose name survived sanitisation as
	// nothing, so every one of its tools is called "mcp__<tool>". Such names
	// collide across services, and the loser is dropped with only a log line
	// to show for it.
	NameDegraded bool `json:"name_degraded,omitempty"`
}

// ListAgentTools godoc
// @Summary      列出 MCP 服务的工具及其在 agent 配置中的名字
// @Description  返回每个工具在 allowed_tools 里应当填写的名称
// @Tags         MCP服务
// @Produce      json
// @Param        id path string true "MCP服务ID"
// @Success      200 {object} map[string]interface{}
// @Router       /mcp-services/{id}/agent-tools [get]
func (h *MCPAgentToolsHandler) ListAgentTools(c *gin.Context) {
	ctx := c.Request.Context()
	serviceID := c.Param("id")

	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}

	svc, err := h.mcpServiceService.GetMCPServiceByID(ctx, tenantID, serviceID)
	if err != nil || svc == nil {
		c.Error(errors.NewNotFoundError("MCP service not found"))
		return
	}

	mcpTools, err := h.mcpServiceService.GetMCPServiceTools(ctx, tenantID, serviceID)
	if err != nil {
		logger.Warnf(ctx, "[MCPAgentTools] listing tools for service %s failed: %v", svc.Name, err)
		c.Error(errors.NewInternalServerError("Failed to get MCP service tools: " + err.Error()))
		return
	}

	views := make([]agentToolView, 0, len(mcpTools))
	degraded := false
	for _, t := range mcpTools {
		if t == nil {
			continue
		}
		// Ask a real MCPTool for its name rather than reimplementing the rule:
		// the transformation lives in one place and this cannot drift from it.
		probe := tools.NewMCPTool(svc, t, nil, nil, 0)
		name := probe.Name()
		// "mcp_" + "" + "_" + tool means the service name sanitised to nothing.
		if len(name) > 4 && name[4] == '_' {
			degraded = true
		}
		views = append(views, agentToolView{
			ToolName:     t.Name,
			RegistryName: name,
			Description:  t.Description,
		})
	}
	if degraded {
		for i := range views {
			views[i].NameDegraded = true
		}
		logger.Warnf(ctx,
			"[MCPAgentTools] service %q sanitises to an empty prefix; its tools are named mcp__<tool> "+
				"and will collide with any other such service that exposes a tool of the same name",
			svc.Name)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    views,
	})
}
