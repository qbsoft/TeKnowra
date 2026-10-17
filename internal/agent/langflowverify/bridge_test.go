package langflowverify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/einoruntime"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestBridgeRejectsForeignOrExpiredTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := &bridge{ctx: ctx, token: "task-secret"}
	for _, auth := range []string{"", "Bearer another-task"} {
		req := httptest.NewRequest(http.MethodPost, "/knowledge", strings.NewReader(`{"query":"contract"}`))
		req.Header.Set("Authorization", auth)
		res := httptest.NewRecorder()
		b.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("foreign task accepted: %d", res.Code)
		}
	}
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/knowledge", strings.NewReader(`{"query":"contract"}`))
	req.Header.Set("Authorization", "Bearer task-secret")
	res := httptest.NewRecorder()
	b.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expired task accepted: %d", res.Code)
	}
}

func TestKnowledgeDoesNotRetrieveAfterRevocation(t *testing.T) {
	called := false
	b := &bridge{ctx: context.Background(), token: "task-secret", opts: einoruntime.Options{
		Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, context.Canceled },
		Retrieve: func(context.Context, types.SearchTargets, []string) ([]*types.SearchResult, error) {
			called = true
			return nil, nil
		},
	}}
	req := httptest.NewRequest(http.MethodPost, "/knowledge", strings.NewReader(`{"query":"contract"}`))
	req.Header.Set("Authorization", "Bearer task-secret")
	res := httptest.NewRecorder()
	b.ServeHTTP(res, req)
	if res.Code == 200 || called {
		t.Fatalf("revoked retrieval executed: code=%d called=%t", res.Code, called)
	}
}

func TestToolRejectsIdentityInjectionAndWriteTools(t *testing.T) {
	called := false
	b := &bridge{ctx: context.Background(), token: "task-secret", opts: einoruntime.Options{
		Revalidate:   func(context.Context) (types.SearchTargets, error) { return nil, nil },
		CallReadOnly: func(context.Context, einoruntime.Invocation) (map[string]any, error) { called = true; return nil, nil },
	}}
	for _, body := range []string{`{"tool":"delete_customer","args":{}}`, `{"tool":"find_customer","args":{},"user_id":"admin"}`} {
		req := httptest.NewRequest(http.MethodPost, "/tool", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer task-secret")
		res := httptest.NewRecorder()
		b.ServeHTTP(res, req)
		if res.Code == 200 || called {
			t.Fatalf("untrusted tool accepted: code=%d", res.Code)
		}
	}
}
