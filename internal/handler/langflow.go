package handler

import (
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
	"time"
)

type LangflowHandler struct {
	catalog  *service.LangflowCatalogService
	releases *service.LangflowReleaseService
}

func NewLangflowHandler(catalog *service.LangflowCatalogService, releases *service.LangflowReleaseService) *LangflowHandler {
	return &LangflowHandler{catalog: catalog, releases: releases}
}
func (h *LangflowHandler) Info(c *gin.Context) {
	agent, authErr := h.releases.EditableAgent(c.Request.Context(), c.Param("id"))
	if authErr != nil {
		c.JSON(403, gin.H{"success": false, "message": authErr.Error()})
		return
	}
	binding, bound, managed := h.releases.DraftMetadata(agent.ID)
	if !bound || binding.WorkspaceID != agent.TenantID {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"bound": false, "setup_enabled": agent.Config.ExecutionEngine == "langflow" && h.releases.Enabled(), "release_enabled": h.releases.Enabled()}})
		return
	}
	c.Header("Cache-Control", "no-store")
	data := gin.H{"bound": true, "setup_enabled": h.releases.Enabled(), "managed": managed, "flow_id": binding.FlowID, "release_enabled": h.releases.Enabled(), "structured_scripts": binding.StructuredScripts}
	if h.releases.Enabled() {
		if active, ok := h.releases.ActiveMetadata(agent.ID); ok && active.WorkspaceID == agent.TenantID {
			data["active_release_id"] = active.ID
			data["active_release_label"] = active.Label
		}
	}
	c.JSON(200, gin.H{"success": true, "data": data})
}

func (h *LangflowHandler) DraftOptions(c *gin.Context) {
	rows, err := h.releases.DraftOptions(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(403, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": rows})
}
func (h *LangflowHandler) SetupDraft(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256*1024)
	var in service.LangflowDraftRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || decoder.Decode(new(any)) != io.EOF {
		c.JSON(400, gin.H{"success": false, "message": "流程配置格式无效"})
		return
	}
	b, err := h.releases.SetupDraft(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"flow_id": b.FlowID, "structured_scripts": b.StructuredScripts}})
}

type langflowReleaseView struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	FlowID      string    `json:"flow_id"`
	CreatedAt   time.Time `json:"created_at"`
	GraphDigest string    `json:"graph_digest"`
}

func releaseView(r langflowrelease.Release) langflowReleaseView {
	return langflowReleaseView{r.ID, r.Label, r.Binding.FlowID, r.CreatedAt, r.GraphDigest}
}
func (h *LangflowHandler) ListReleases(c *gin.Context) {
	rows, active, err := h.releases.List(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(403, gin.H{"success": false, "message": err.Error()})
		return
	}
	result := []langflowReleaseView{}
	for _, r := range rows {
		result = append(result, releaseView(r))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"success": true, "data": gin.H{"releases": result, "active_release_id": active}})
}
func (h *LangflowHandler) Publish(c *gin.Context) {
	var body struct {
		Label string `json:"label"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(400, gin.H{"success": false, "message": "发布名称无效"})
		return
	}
	r, err := h.releases.Publish(c.Request.Context(), c.Param("id"), body.Label)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"success": true, "data": releaseView(r)})
}
func (h *LangflowHandler) Activate(c *gin.Context) {
	if err := h.releases.Activate(c.Request.Context(), c.Param("id"), c.Param("release_id")); err != nil {
		c.JSON(400, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}
func (h *LangflowHandler) Create(c *gin.Context) {
	token, expires, err := h.catalog.Create(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(403, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"success": true, "data": gin.H{"directory_url": "http://127.0.0.1:8080/api/v1/langflow/design/catalog", "directory_ticket": token, "expires_at": expires, "scope": "directory_metadata_only"}})
}
func (h *LangflowHandler) Directory(c *gin.Context) {
	token := c.GetHeader("X-TeKnowra-Design-Ticket")
	if len(token) != 64 || strings.ContainsAny(token, "\r\n") {
		c.JSON(401, gin.H{"success": false, "message": "目录连接无效"})
		return
	}
	catalog, err := h.catalog.Read(c.Request.Context(), token)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "message": "目录连接失效，请从TeKnowra重新连接"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"success": true, "data": catalog})
}
