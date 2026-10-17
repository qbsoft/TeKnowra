package handler

import (
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *MessageHandler) GetExecutionDiagnostics(c *gin.Context) {
	ctx := c.Request.Context()
	p, _ := types.PrincipalFromContext(ctx)
	if p.Type != types.PrincipalWebUser || types.CallerFromContext(ctx).Role.Level() < types.TenantRoleContributor.Level() {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅授权研发可查看本人会话的执行诊断"})
		return
	}
	m, err := h.MessageService.GetMessage(ctx, c.Param("session_id"), c.Param("id"))
	if err != nil || m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "消息不存在或无权访问"})
		return
	}
	versions := []gin.H{}
	for _, s := range m.AgentSteps {
		versions = append(versions, gin.H{"run_id": s.RunID, "node": s.NodeKey, "definition": s.DefinitionVersion, "implementation": s.ImplementationVersion})
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"diagnostics": m.ExecutionContext.ExecutionDiagnostics, "versions": versions, "historical_implementation_unknown": len(versions) == 0 || versions[0]["implementation"] == ""}})
}
