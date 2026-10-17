package handler

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Shared resources belong to the source workspace, but the prompt belongs to
// the employee's workspace. Resolving either must not change the other scope.
func TestSharedMCPOAuthPromptUsesCallerWorkspace(t *testing.T) {
	for _, action := range []string{"authorize", "cancel", "skip"} {
		t.Run(action, func(t *testing.T) {
			principal := types.Principal{Type: types.PrincipalWebUser, ID: "sales01"}
			repo := &mcpOAuthTestRepo{tokens: map[string]*types.MCPOAuthToken{
				mcpOAuthTestTokenKey(testSourceTenant, principal, testServiceID): {
					TenantID: testSourceTenant, ServiceID: testServiceID,
					PrincipalType: principal.Type, PrincipalID: principal.ID,
					AccessToken: "synthetic-test-only", ExpiresAt: time.Now().Add(time.Hour),
				},
			}}
			gate := approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 1}}, nil, nil)
			h := &MCPOAuthHandler{
				gate: gate, agentShare: &stubAgentShare{agent: sharedAgent("selected", testServiceID)},
				oauth: mcp.NewOAuthManager(repo, mcpOAuthTestServiceRepo{service: &types.MCPService{ID: testServiceID}}, nil),
			}
			bus := event.NewEventBus()
			bus.On(event.EventMCPOAuthRequired, func(_ context.Context, e event.Event) error {
				pendingID := e.Data.(event.MCPOAuthRequiredData).PendingID
				c := oauthTestContext(sharedQuery)
				ctx := types.WithCaller(c.Request.Context(), types.Caller{TenantID: testCallerTenant, UserID: principal.ID})
				c.Request = c.Request.WithContext(types.WithPrincipal(ctx, principal))
				c.Params = gin.Params{{Key: "pending_id", Value: pendingID}}
				if action == "skip" {
					h.CancelMCPOAuth(c)
				} else {
					body := `{"service_id":"` + testServiceID + `","decision":"` + action + `"}`
					c.Request.Body = io.NopCloser(strings.NewReader(body))
					c.Request.Header.Set("Content-Type", "application/json")
					h.ResolveMCPOAuth(c)
				}
				for _, err := range c.Errors {
					t.Errorf("shared OAuth %s: %v", action, err.Err)
				}
				return nil
			})
			decision, err := gate.RequestOAuthAndWait(context.Background(), approval.OAuthPendingRequest{
				TenantID: testCallerTenant, UserID: principal.StorageID(),
				SessionID: "shared-session", AssistantMessageID: "message", ServiceID: testServiceID, EventBus: bus,
			})
			require.NoError(t, err)
			require.False(t, decision.TimedOut, "the caller's prompt must resolve")
			require.Equal(t, action == "authorize", decision.Approved)
		})
	}
}
