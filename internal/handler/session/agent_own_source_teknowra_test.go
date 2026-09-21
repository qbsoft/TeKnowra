package session

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

func ownSourceTestContext(tenant uint64) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/x", nil)
	if tenant != 0 {
		c.Set(types.TenantIDContextKey.String(), tenant)
	}
	return c
}

func TestOwnTenantIsNotAShareSource(t *testing.T) {
	tests := []struct {
		name   string
		tenant uint64
		source uint64
		want   uint64
	}{
		{"来源空间就是自己的空间：当作自己的智能体", 10000, 10000, 0},
		{"来源是别的空间：一字不改，上游对共享的校验原样生效", 10002, 10000, 10000},
		{"没带来源空间：不变", 10000, 0, 0},
		{"上下文里没有当前空间：不猜，原样返回", 0, 10000, 10000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ownTenantIsNotAShareSource(ownSourceTestContext(tt.tenant), tt.source); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
	if got := ownTenantIsNotAShareSource(nil, 7); got != 7 {
		t.Errorf("nil context must leave the value alone, got %d", got)
	}
}

// 钩子在上游文件 qa.go 的 resolveAgent 里只有一行，而且必须在任何使用 sourceTenantID 的代码之前。
// 被合并冲掉的话：从共享空间选了自己智能体的人，每次提问都是 404。
func TestResolveAgentNormalizesOwnTenantSource(t *testing.T) {
	src, err := os.ReadFile("qa.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, "func (h *Handler) resolveAgent(")
	if start < 0 {
		t.Fatal("resolveAgent not found (renamed upstream?)")
	}
	body := text[start:]
	hook := strings.Index(body, "sourceTenantID = ownTenantIsNotAShareSource(c, sourceTenantID)")
	if hook < 0 {
		t.Fatal("resolveAgent no longer normalizes an own-workspace source id")
	}
	if first := strings.Index(body, "GetSharedAgentForTenant("); first < 0 || hook > first {
		t.Error("the hook must run before the shared-agent lookup")
	}
}
