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
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

const (
	testCallerTenant uint64 = 10002
	testSourceTenant uint64 = 10000
	testAgentID             = "agent-ask-data"
	testServiceID           = "svc-crm-mcp"
)

// stubAgentShare 只实现本测试用到的一个方法；其余方法走内嵌的 nil 接口，
// 一旦被意外调用会直接 panic —— 正好暴露"多调了不该调的东西"。
type stubAgentShare struct {
	interfaces.AgentShareService
	agent *types.CustomAgent
	err   error

	gotTenant uint64
	gotAgent  string
	gotSource []uint64
	calls     int
}

func (s *stubAgentShare) GetSharedAgentForTenant(
	_ context.Context, tenantID uint64, _ types.TenantRole, agentID string, sourceTenantID ...uint64,
) (*types.CustomAgent, error) {
	s.calls++
	s.gotTenant, s.gotAgent, s.gotSource = tenantID, agentID, sourceTenantID
	return s.agent, s.err
}

func sharedAgent(mode string, services ...string) *types.CustomAgent {
	a := &types.CustomAgent{ID: testAgentID, TenantID: testSourceTenant}
	a.Config.MCPSelectionMode = mode
	a.Config.MCPServices = services
	return a
}

func oauthTestContext(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/x?"+query, nil)
	c.Set(types.TenantIDContextKey.String(), testCallerTenant)
	return c
}

const sharedQuery = "agent_id=" + testAgentID + "&agent_source_tenant_id=10000"

func TestOAuthTenantFor(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		share      *stubAgentShare
		serviceID  string
		wantTenant uint64
		wantErr    bool
		wantCalls  int
	}{
		{
			name:       "不带参数：与上游行为一致，用请求方自己的空间，且不碰共享服务",
			query:      "",
			share:      &stubAgentShare{},
			serviceID:  testServiceID,
			wantTenant: testCallerTenant,
			wantCalls:  0,
		},
		{
			name:       "只带一半参数：当作没带",
			query:      "agent_id=" + testAgentID,
			share:      &stubAgentShare{},
			serviceID:  testServiceID,
			wantTenant: testCallerTenant,
			wantCalls:  0,
		},
		{
			name:       "源空间就是自己的空间：不需要共享校验",
			query:      "agent_id=" + testAgentID + "&agent_source_tenant_id=10002",
			share:      &stubAgentShare{},
			serviceID:  testServiceID,
			wantTenant: testCallerTenant,
			wantCalls:  0,
		},
		{
			name:       "已共享且智能体选中了该服务：换成源空间",
			query:      sharedQuery,
			share:      &stubAgentShare{agent: sharedAgent("selected", "other", testServiceID)},
			serviceID:  testServiceID,
			wantTenant: testSourceTenant,
			wantCalls:  1,
		},
		{
			name:       "已共享且智能体用全部服务：换成源空间",
			query:      sharedQuery,
			share:      &stubAgentShare{agent: sharedAgent("all")},
			serviceID:  testServiceID,
			wantTenant: testSourceTenant,
			wantCalls:  1,
		},
		{
			name:      "没有被共享：拒绝，绝不回退到源空间",
			query:     sharedQuery,
			share:     &stubAgentShare{err: errors.New("not shared")},
			serviceID: testServiceID,
			wantErr:   true,
			wantCalls: 1,
		},
		{
			name:      "共享了，但智能体没用这个服务：拒绝（不能拿共享智能体当钥匙去授权源空间里别的服务）",
			query:     sharedQuery,
			share:     &stubAgentShare{agent: sharedAgent("selected", "other")},
			serviceID: testServiceID,
			wantErr:   true,
			wantCalls: 1,
		},
		{
			name:      "共享了，但智能体不挂任何 MCP：拒绝",
			query:     sharedQuery,
			share:     &stubAgentShare{agent: sharedAgent("none")},
			serviceID: testServiceID,
			wantErr:   true,
			wantCalls: 1,
		},
		{
			name:      "共享服务返回的智能体不属于声称的源空间：拒绝",
			query:     sharedQuery,
			share:     &stubAgentShare{agent: &types.CustomAgent{ID: testAgentID, TenantID: 99999}},
			serviceID: testServiceID,
			wantErr:   true,
			wantCalls: 1,
		},
		{
			name:      "源空间参数不是数字：拒绝",
			query:     "agent_id=" + testAgentID + "&agent_source_tenant_id=abc",
			share:     &stubAgentShare{},
			serviceID: testServiceID,
			wantErr:   true,
			wantCalls: 0,
		},
		{
			name:       "调用点还不知道服务（取消）：只校验共享关系",
			query:      sharedQuery,
			share:      &stubAgentShare{agent: sharedAgent("selected", "other")},
			serviceID:  "",
			wantTenant: testSourceTenant,
			wantCalls:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &MCPOAuthHandler{agentShare: tt.share}
			got, err := h.oauthTenantFor(oauthTestContext(tt.query), tt.serviceID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && got != 0 {
				t.Errorf("rejected request must not yield a tenant, got %d", got)
			}
			if !tt.wantErr && got != tt.wantTenant {
				t.Errorf("tenant = %d, want %d", got, tt.wantTenant)
			}
			if tt.share.calls != tt.wantCalls {
				t.Errorf("share service calls = %d, want %d", tt.share.calls, tt.wantCalls)
			}
			if tt.wantCalls == 1 {
				if tt.share.gotTenant != testCallerTenant || tt.share.gotAgent != testAgentID ||
					len(tt.share.gotSource) != 1 || tt.share.gotSource[0] != testSourceTenant {
					t.Errorf("share check called with tenant=%d agent=%q source=%v",
						tt.share.gotTenant, tt.share.gotAgent, tt.share.gotSource)
				}
			}
		})
	}
}

func TestOAuthTenantForWithoutShareService(t *testing.T) {
	h := &MCPOAuthHandler{}
	if _, err := h.oauthTenantFor(oauthTestContext(sharedQuery), testServiceID); err == nil {
		t.Fatal("cross-workspace request must be rejected when the share service is unavailable")
	}
}

// 钩子挂在上游文件 mcp_oauth.go 里，每个接口一行。合并上游时这些行若被冲掉，
// 跨空间共享的授权会悄悄退回"按请求方空间找服务"——功能坏了但不报错。这条测试盯着它。
func TestMCPOAuthEndpointsUseSharedAgentTenant(t *testing.T) {
	src, err := os.ReadFile("mcp_oauth.go")
	if err != nil {
		t.Fatalf("read mcp_oauth.go: %v", err)
	}
	text := string(src)
	for _, fn := range []string{"AuthorizeURL", "Status", "Revoke", "ResolveMCPOAuth", "CancelMCPOAuth"} {
		start := strings.Index(text, "func (h *MCPOAuthHandler) "+fn+"(c *gin.Context) {")
		if start < 0 {
			t.Errorf("%s: handler not found (renamed upstream?)", fn)
			continue
		}
		body := text[start:]
		if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
			body = body[:next+1]
		}
		if !strings.Contains(body, "h.oauthTenantFor(c, ") {
			t.Errorf("%s no longer resolves its workspace through oauthTenantFor", fn)
		}
		if strings.Contains(body, "tenantID := c.GetUint64(types.TenantIDContextKey.String())") {
			t.Errorf("%s reads the caller workspace directly again", fn)
		}
	}
}
