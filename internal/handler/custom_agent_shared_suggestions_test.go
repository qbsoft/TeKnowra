package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

func suggestionCtx(query string, tenant uint64) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/x?"+query, nil)
	c.Set(types.TenantIDContextKey.String(), tenant)
	return c
}

func TestSharedAgentSuggestionScope(t *testing.T) {
	callerKB := []string{"kb-of-caller"}
	callerDocs := []string{"doc-1"}
	callerTags := []types.TagScope{{KnowledgeBaseID: "kb-x", TagIDs: []string{"t1"}}}

	t.Run("不带参数：与上游一致，范围原样保留、空间不变、不碰共享服务", func(t *testing.T) {
		share := &stubAgentShare{}
		h := &CustomAgentHandler{agentShare: share}
		ctx, kb, docs, tags, err := h.sharedAgentSuggestionScope(
			suggestionCtx("", testCallerTenant), context.Background(), testAgentID, callerKB, callerDocs, callerTags)
		if err != nil || len(kb) != 1 || len(docs) != 1 || len(tags) != 1 || share.calls != 0 {
			t.Fatalf("err=%v kb=%v docs=%v tags=%v calls=%d", err, kb, docs, tags, share.calls)
		}
		if _, ok := types.TenantIDFromContext(ctx); ok {
			t.Error("tenant must not be injected when no source is given")
		}
	})

	t.Run("来源就是自己的空间：同上", func(t *testing.T) {
		share := &stubAgentShare{}
		h := &CustomAgentHandler{agentShare: share}
		_, kb, _, _, err := h.sharedAgentSuggestionScope(
			suggestionCtx("agent_source_tenant_id=10002", testCallerTenant), context.Background(), testAgentID, callerKB, nil, nil)
		if err != nil || len(kb) != 1 || share.calls != 0 {
			t.Fatalf("err=%v kb=%v calls=%d", err, kb, share.calls)
		}
	})

	t.Run("确实共享给了我：换到来源空间，并丢掉我自带的知识库范围", func(t *testing.T) {
		share := &stubAgentShare{agent: sharedAgent("selected", testServiceID)}
		h := &CustomAgentHandler{agentShare: share}
		ctx, kb, docs, tags, err := h.sharedAgentSuggestionScope(
			suggestionCtx("agent_source_tenant_id=10000", testCallerTenant), context.Background(), testAgentID, callerKB, callerDocs, callerTags)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := types.TenantIDFromContext(ctx); got != testSourceTenant {
			t.Errorf("tenant = %d, want source %d", got, testSourceTenant)
		}
		if kb != nil || docs != nil || tags != nil {
			t.Errorf("caller-supplied scopes must be dropped, got kb=%v docs=%v tags=%v", kb, docs, tags)
		}
		if share.gotTenant != testCallerTenant || share.gotAgent != testAgentID ||
			len(share.gotSource) != 1 || share.gotSource[0] != testSourceTenant {
			t.Errorf("share check called with tenant=%d agent=%q source=%v", share.gotTenant, share.gotAgent, share.gotSource)
		}
	})

	t.Run("没有共享给我：拒绝，绝不换空间", func(t *testing.T) {
		h := &CustomAgentHandler{agentShare: &stubAgentShare{err: errors.New("not shared")}}
		ctx, _, _, _, err := h.sharedAgentSuggestionScope(
			suggestionCtx("agent_source_tenant_id=10000", testCallerTenant), context.Background(), testAgentID, callerKB, nil, nil)
		if err == nil {
			t.Fatal("expected rejection")
		}
		if _, ok := types.TenantIDFromContext(ctx); ok {
			t.Error("a rejected request must not switch workspace")
		}
	})

	t.Run("共享服务返回的智能体不属于声称的来源空间：拒绝", func(t *testing.T) {
		h := &CustomAgentHandler{agentShare: &stubAgentShare{agent: &types.CustomAgent{ID: testAgentID, TenantID: 99999}}}
		if _, _, _, _, err := h.sharedAgentSuggestionScope(
			suggestionCtx("agent_source_tenant_id=10000", testCallerTenant), context.Background(), testAgentID, nil, nil, nil); err == nil {
			t.Fatal("expected rejection")
		}
	})

	t.Run("来源参数不是数字：拒绝", func(t *testing.T) {
		h := &CustomAgentHandler{agentShare: &stubAgentShare{}}
		if _, _, _, _, err := h.sharedAgentSuggestionScope(
			suggestionCtx("agent_source_tenant_id=abc", testCallerTenant), context.Background(), testAgentID, nil, nil, nil); err == nil {
			t.Fatal("expected rejection")
		}
	})

	t.Run("没有共享服务：跨空间请求一律拒绝", func(t *testing.T) {
		h := &CustomAgentHandler{}
		if _, _, _, _, err := h.sharedAgentSuggestionScope(
			suggestionCtx("agent_source_tenant_id=10000", testCallerTenant), context.Background(), testAgentID, nil, nil, nil); err == nil {
			t.Fatal("expected rejection")
		}
	})
}

// 钩子在上游文件 custom_agent.go 里一行，必须在调用服务之前。被合并冲掉的话，共享智能体的开场问题又会消失。
func TestGetSuggestedQuestionsUsesSharedAgentScope(t *testing.T) {
	src, err := os.ReadFile("custom_agent.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, "func (h *CustomAgentHandler) GetSuggestedQuestions(c *gin.Context) {")
	if start < 0 {
		t.Fatal("GetSuggestedQuestions not found (renamed upstream?)")
	}
	body := text[start:]
	if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
		body = body[:next+1]
	}
	hook := strings.Index(body, "h.sharedAgentSuggestionScope(c, ctx, id, kbIDs, knowledgeIDs, tagScopes)")
	call := strings.Index(body, "h.service.GetSuggestedQuestions(")
	if hook < 0 || call < 0 || hook > call {
		t.Fatal("GetSuggestedQuestions must resolve the shared-agent scope before calling the service")
	}
}
