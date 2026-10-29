// Throwaway verification probe. Copy into internal/agent/tools only while testing.
// No product runtime or stored user credentials are changed.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	"github.com/Tencent/WeKnora/internal/agent/policy"
	"github.com/Tencent/WeKnora/internal/agent/runbudget"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	imcp "github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/cloudwego/eino/compose"
	sdk "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

// Unexpected repository methods panic via the embedded nil interface: this
// probe must not silently treat a refresh or client-registration call as tested.
type resumeOAuthRepo struct {
	interfaces.MCPOAuthRepository
	mu     sync.Mutex
	tokens map[string]*types.MCPOAuthToken
}

func (r *resumeOAuthRepo) GetClient(context.Context, uint64, string) (*types.MCPOAuthClient, error) {
	return nil, nil
}
func (r *resumeOAuthRepo) GetTokenForPrincipal(_ context.Context, tenant uint64, p types.Principal, service string) (*types.MCPOAuthToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v := r.tokens[fmt.Sprintf("%d/%s/%s", tenant, p.StorageID(), service)]; v != nil {
		c := *v
		return &c, nil
	}
	return nil, nil
}
func (r *resumeOAuthRepo) grant(user string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens["7/web_user:"+user+"/probe"] = &types.MCPOAuthToken{TenantID: 7, ServiceID: "probe", UserID: user, PrincipalType: types.PrincipalWebUser, PrincipalID: user, AccessToken: "synthetic-" + user, TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour)}
}

func (r *resumeOAuthRepo) DeleteTokenForPrincipal(_ context.Context, tenant uint64, p types.Principal, service string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tokens, fmt.Sprintf("%d/%s/%s", tenant, p.StorageID(), service))
	return nil
}

type resumePolicyGate struct {
	*approval.Gate
	disabled atomic.Bool
	scenario string
}

// Upstream now decides via Action+policy.Decide; Always forces Ask so the
// TeKnowra pin/OAuth scenarios still exercise RequestAndWait.
func (g *resumePolicyGate) Action(context.Context, uint64, string, *types.MCPTool) policy.Action {
	if strings.Contains(g.scenario, "approval_then_oauth") || g.scenario == "modified_args" {
		return policy.Action{Risk: policy.RiskWrite, Always: true}
	}
	return policy.Action{Risk: policy.RiskWrite}
}
func (g *resumePolicyGate) RequestAndWait(ctx context.Context, _ approval.PendingRequest) (approval.Decision, error) {
	if g.scenario == "slow_approval_then_oauth" {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return approval.Decision{}, ctx.Err()
		}
	}
	if g.scenario == "modified_args" {
		return approval.Decision{Approved: true, ModifiedArgs: json.RawMessage(`{"request":"tampered"}`)}, nil
	}
	return approval.Decision{Approved: true}, nil
}

func (g *resumePolicyGate) IsEnabled(context.Context, uint64, string, string) (bool, error) {
	return !g.disabled.Load(), nil
}

func TestMCPOAuthResume(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	for _, scenario := range []string{"approval_then_oauth", "slow_approval_then_oauth", "modified_args", "slow_authorization", "slow_budget", "slow_discovery", "disabled_during_wait", "expired_reauth", "cached_revoked_reauth", "approve", "duplicate_resume", "cancel", "cancel_then_login", "reject", "timeout", "other_user", "run_deadline", "http_forbidden"} {
		t.Run(scenario, func(t *testing.T) {
			repo := &resumeOAuthRepo{tokens: map[string]*types.MCPOAuthToken{}}
			if scenario == "http_forbidden" {
				repo.grant("alice")
			}
			if scenario == "other_user" {
				repo.grant("bob")
			}
			var calls, prompts atomic.Int32
			var savedPending string
			s := server.NewMCPServer("read-only-probe", "1", server.WithToolCapabilities(false))
			s.AddTool(sdk.NewTool("read_identity", sdk.WithString("request", sdk.Required())), func(ctx context.Context, req sdk.CallToolRequest) (*sdk.CallToolResult, error) {
				calls.Add(1)
				if req.GetArguments()["request"] != "same-input" {
					return sdk.NewToolResultError("input changed"), nil
				}
				return sdk.NewToolResultText("synthetic identity read"), nil
			})
			transport := server.NewStreamableHTTPServer(s, server.WithStateLess(true))
			endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario == "http_forbidden" || r.Header.Get("Authorization") != "Bearer synthetic-alice" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				transport.ServeHTTP(w, r)
			}))
			defer endpoint.Close()
			manager := imcp.NewMCPManager(repo)
			defer manager.Shutdown()
			svc := &types.MCPService{ID: "probe", TenantID: 7, Name: "readonly-probe", Enabled: true, URL: &endpoint.URL, TransportType: types.MCPTransportHTTPStreamable, AuthConfig: &types.MCPAuthConfig{AuthType: "oauth"}}
			gate := approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 1}}, nil, nil)
			policy := &resumePolicyGate{Gate: gate, scenario: scenario}
			bus := event.NewEventBus()
			root, cancel := context.WithCancel(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7)))
			defer cancel()
			if scenario == "slow_budget" || scenario == "slow_approval_then_oauth" {
				var closeBudget context.CancelFunc
				root, closeBudget = runbudget.New(root, runbudget.Limits{Active: 80 * time.Millisecond, Wait: 500 * time.Millisecond, Wall: time.Second, MaxWaits: 1})
				defer closeBudget()
			}
			if scenario == "run_deadline" {
				var deadlineCancel context.CancelFunc
				root, deadlineCancel = context.WithTimeout(root, 25*time.Millisecond)
				defer deadlineCancel()
			}
			root = types.WithPrincipal(root, types.Principal{Type: types.PrincipalWebUser, ID: "alice"})
			root = context.WithValue(root, types.UserIDContextKey, "alice")
			bus.On(event.EventMCPOAuthRequired, func(_ context.Context, e event.Event) error {
				d := e.Data.(event.MCPOAuthRequiredData)
				prompts.Add(1)
				savedPending = d.PendingID
				require.Zero(t, calls.Load(), "business tool ran before authorization")
				require.Equal(t, "alice", func() string {
					if p, _ := types.PrincipalFromContext(root); p.ID == "alice" {
						return p.ID
					}
					return "wrong"
				}())
				require.Equal(t, "session", d.SessionID)
				if scenario != "slow_discovery" {
					require.Equal(t, "read_identity", d.MCPToolName)
				}
				require.Equal(t, "call", d.ToolCallID)
				require.ErrorIs(t, gate.Resolve(context.Background(), approval.ResolveRequest{TenantID: 7, UserID: "bob", PendingID: d.PendingID, Kind: types.AgentPromptKindMCPOAuth, Decision: approval.Decision{Approved: true}}), approval.ErrUserMismatch)
				require.ErrorIs(t, gate.Resolve(context.Background(), approval.ResolveRequest{TenantID: 8, UserID: "alice", PendingID: d.PendingID, Kind: types.AgentPromptKindMCPOAuth, Decision: approval.Decision{Approved: true}}), approval.ErrTenantMismatch)
				switch scenario {
				case "approval_then_oauth", "slow_approval_then_oauth", "slow_discovery", "slow_budget", "disabled_during_wait", "slow_authorization", "approve", "duplicate_resume", "expired_reauth", "cached_revoked_reauth":
					if strings.HasPrefix(scenario, "slow_") {
						time.Sleep(100 * time.Millisecond)
					}
					if scenario == "disabled_during_wait" {
						policy.disabled.Store(true)
					}
					repo.grant("alice")
					first := gate.Resolve(context.Background(), approval.ResolveRequest{TenantID: 7, UserID: "alice", PendingID: d.PendingID, Kind: types.AgentPromptKindMCPOAuth, Decision: approval.Decision{Approved: true}})
					if scenario == "duplicate_resume" {
						require.Error(t, gate.Resolve(context.Background(), approval.ResolveRequest{TenantID: 7, UserID: "alice", PendingID: d.PendingID, Kind: types.AgentPromptKindMCPOAuth, Decision: approval.Decision{Approved: true}}), "second resume must be rejected")
					}
					return first
				case "reject", "other_user":
					return gate.Resolve(context.Background(), approval.ResolveRequest{TenantID: 7, UserID: "alice", PendingID: d.PendingID, Kind: types.AgentPromptKindMCPOAuth, Decision: approval.Decision{Approved: false, Reason: "declined"}})
				case "cancel", "cancel_then_login":
					cancel()
				}
				return nil
			})
			if scenario == "expired_reauth" || scenario == "cached_revoked_reauth" {
				repo.grant("alice")
				if scenario == "cached_revoked_reauth" {
					_, warmErr := manager.GetOrCreateClient(root, svc)
					require.NoError(t, warmErr)
					require.NoError(t, repo.DeleteTokenForPrincipal(root, 7, types.Principal{Type: types.PrincipalWebUser, ID: "alice"}, "probe"))
				} else {
					repo.mu.Lock()
					repo.tokens["7/web_user:alice/probe"].ExpiresAt = time.Now().Add(-time.Minute)
					repo.mu.Unlock()
				}
			}
			tool := NewMCPTool(svc, &types.MCPTool{Name: "read_identity", Description: "read only", InputSchema: json.RawMessage(`{"type":"object","properties":{"request":{"type":"string"}}}`)}, manager, policy, 1)
			if scenario == "modified_args" {
				tool.PinArguments(json.RawMessage(`{"request":"same-input"}`))
			}
			graph := compose.NewGraph[string, *types.ToolResult]()
			require.NoError(t, graph.AddLambdaNode("read", compose.InvokableLambda(func(ctx context.Context, input string) (*types.ToolResult, error) {
				ctx = WithToolExecContext(ctx, &ToolExecContext{EventBus: bus, SessionID: "session", AssistantMessageID: "message", UserID: "alice", ToolCallID: "call", ApprovalCtx: root, ExecTimeout: 40 * time.Millisecond})
				if scenario == "slow_discovery" {
					operation, done := context.WithTimeout(ctx, 40*time.Millisecond)
					defer done()
					listed, _, e := loadMCPServiceTools(operation, svc, manager, policy, oauthSessionFromToolExec(ctx, &ToolExecContext{EventBus: bus, SessionID: "session", UserID: "alice", ToolCallID: "call", ApprovalCtx: root, ExecTimeout: time.Second}))
					return &types.ToolResult{Success: e == nil && len(listed) == 1}, e
				}
				args, _ := json.Marshal(map[string]string{"request": input})
				return tool.Execute(ctx, args)
			})))
			require.NoError(t, graph.AddEdge(compose.START, "read"))
			require.NoError(t, graph.AddEdge("read", compose.END))
			runnable, err := graph.Compile(root)
			require.NoError(t, err)
			out, err := runnable.Invoke(root, "same-input")
			if scenario == "approval_then_oauth" || strings.HasPrefix(scenario, "slow_") || scenario == "approve" || scenario == "duplicate_resume" || scenario == "expired_reauth" || scenario == "cached_revoked_reauth" {
				require.NoError(t, err)
				require.True(t, out.Success, out.Error)
				if scenario == "slow_discovery" {
					require.Zero(t, calls.Load())
				} else {
					require.Equal(t, int32(1), calls.Load())
				}
				require.Equal(t, int32(1), prompts.Load())
				require.NotContains(t, out.Output, "synthetic-alice")
			} else {
				if scenario == "cancel_then_login" {
					repo.grant("alice")
					require.Error(t, gate.Resolve(context.Background(), approval.ResolveRequest{TenantID: 7, UserID: "alice", PendingID: savedPending, Kind: types.AgentPromptKindMCPOAuth, Decision: approval.Decision{Approved: true}}), "late login must not revive stopped task")
				}
				require.Zero(t, calls.Load())
				if scenario == "http_forbidden" || scenario == "modified_args" {
					require.Zero(t, prompts.Load(), "403 must not become an OAuth prompt")
				} else {
					require.Positive(t, prompts.Load())
				}
				if err == nil {
					require.False(t, out.Success)
				}
			}
		})
	}
}
