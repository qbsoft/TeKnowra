package handler

import (
	"os"
	"strings"
	"testing"
)

// 钩子挂在上游文件 mcp_oauth.go 的 AuthorizeURL 里，一行。合并上游时若被冲掉，回跳地址会
// 悄悄退回「前端报什么就是什么」——平时不报错，只有用户换个地址进平台时授权才失败。
func TestAuthorizeURLUsesCanonicalRedirectURI(t *testing.T) {
	src, err := os.ReadFile("mcp_oauth.go")
	if err != nil {
		t.Fatalf("read mcp_oauth.go: %v", err)
	}
	text := string(src)
	start := strings.Index(text, "func (h *MCPOAuthHandler) AuthorizeURL(c *gin.Context) {")
	if start < 0 {
		t.Fatal("AuthorizeURL not found (renamed upstream?)")
	}
	body := text[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	hook := strings.Index(body, "h.oauth.CanonicalRedirectURI(")
	if hook < 0 {
		t.Fatal("AuthorizeURL no longer resolves the redirect URI through CanonicalRedirectURI")
	}
	if use := strings.Index(body, "h.oauth.StartAuthorization("); use < 0 || hook > use {
		t.Error("the canonical redirect URI must be resolved before StartAuthorization is called")
	}
}
