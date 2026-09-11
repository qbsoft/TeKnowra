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

// MCPAgentToolsHandler lists an MCP service's tools under the keys an agent's
// allowed_tools must contain to grant them.
//
// The existing /mcp-services/:id/tools endpoint returns what the MCP server
// itself reports — "send_email". What goes into an agent's allowed_tools is
// the grant key — "mcp:<service_id>:send_email". Deriving it here rather than
// in the browser keeps one implementation of the rule (tools.AgentMCPToolKey).
//
// Earlier versions exposed the registry name ("mcp_mail_send_email") instead.
// That name is derived from the service's display name, which sanitises
// lossily and now embeds a schema hash — both wrong for a stored grant; see
// tools.AgentMCPToolKey for the full reasoning.
type MCPAgentToolsHandler struct {
	mcpServiceService interfaces.MCPServiceService
}

// NewMCPAgentToolsHandler constructs the handler.
func NewMCPAgentToolsHandler(svc interfaces.MCPServiceService) *MCPAgentToolsHandler {
	return &MCPAgentToolsHandler{mcpServiceService: svc}
}

// agentToolView pairs what the MCP server calls a tool with the grant key an
// agent config uses to allow it.
type agentToolView struct {
	// ToolName is the name the MCP server reports.
	ToolName string `json:"tool_name"`
	// AuthorizationKey is what goes into an agent's allowed_tools:
	// "mcp:<service_id>:<tool_name>".
	AuthorizationKey string `json:"authorization_key"`
	Description      string `json:"description,omitempty"`
}

// ListAgentTools godoc
// @Summary      列出 MCP 服务的工具及其在 agent 配置中的授权键
// @Description  返回每个工具在 allowed_tools 里应当填写的键（mcp:<服务ID>:<工具名>）
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
	for _, t := range mcpTools {
		if t == nil {
			continue
		}
		views = append(views, agentToolView{
			ToolName:         t.Name,
			AuthorizationKey: tools.AgentMCPToolKey(serviceID, t.Name),
			Description:      t.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    views,
	})
}
