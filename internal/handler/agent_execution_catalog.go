package handler

import (
	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/agent/einoruntime"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *CustomAgentHandler) GetExecutionCatalog(c *gin.Context) {
	definitions := make([]map[string]any, 0)
	for _, id := range einodef.IDs() {
		d, _ := einodef.Lookup(id)
		definitions = append(definitions, map[string]any{"id": id, "name": d.Name, "knowledge": d.Knowledge, "tools": d.ReadOnlyTools, "implementation_version": einoruntime.ImplementationVersion(id)})
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"engines": []gin.H{
		{"id": "native", "name": "TeKnowra 原生", "unsupported_sections": []string{}, "unsupported_settings": []string{}},
		{"id": "langflow", "name": "Langflow 可视编排", "unsupported_sections": []string{"websearch"}, "unsupported_settings": []string{"memory_enabled", "image_upload_enabled", "web_search_enabled"}},
		{"id": "eino", "name": "Eino 代码编排", "unsupported_sections": []string{"conversation", "websearch", "multimodal", "skills", "share"}, "unsupported_settings": []string{"memory_enabled", "image_upload_enabled", "web_search_enabled", "multi_turn_enabled", "allow_session_attachments", "skills_selection_mode", "sandbox_config_id"}},
	}, "eino_definitions": definitions}})
}
