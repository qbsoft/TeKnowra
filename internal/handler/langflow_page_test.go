package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLangflowDraftRejectsRequestAuthorityAndTrailingJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{
		`{"mode":"create","workspace_id":999}`,
		`{"mode":"create","models":["foreign"]}`,
		`{"mode":"create","node_key":"fake"}`,
		`{"mode":"create"}{"mode":"sync"}`,
		strings.Repeat(" ", 256*1024) + `{}`,
	} {
		t.Run(body[:min(len(body), 70)], func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = httptest.NewRequest("POST", "/agents/a/langflow/draft", strings.NewReader(body))
			// No service is supplied: invalid authority must fail before reaching it.
			(&LangflowHandler{}).SetupDraft(ctx)
			if response.Code != 400 {
				t.Fatalf("unexpected status: %d", response.Code)
			}
		})
	}
}
