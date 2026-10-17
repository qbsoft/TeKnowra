package handler

import (
	"context"
	"fmt"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

type diagnosticMessageService struct {
	interfaces.MessageService
	calls int
}

func (s *diagnosticMessageService) GetMessage(_ context.Context, sid, id string) (*types.Message, error) {
	s.calls++
	if sid != "mine" || id != "m" {
		return nil, fmt.Errorf("not owned")
	}
	return &types.Message{ID: "m", AgentSteps: types.AgentSteps{{DefinitionVersion: "contract-v1", ImplementationVersion: "source-hash"}}, ExecutionContext: types.MessageExecutionContext{ExecutionDiagnostics: []types.ExecutionDiagnostic{types.NewExecutionDiagnostic("eino", "validate", "fact_audit_rejected", []string{"amount mismatch"})}}}, nil
}
func TestExecutionDiagnosticsRequireWebDeveloperAndOwnedMessage(t *testing.T) {
	for _, tc := range []struct {
		principal string
		role      types.TenantRole
		session   string
		status    int
	}{
		{types.PrincipalWebUser, types.TenantRoleContributor, "mine", 200},
		{types.PrincipalWebUser, types.TenantRoleViewer, "mine", 403},
		{types.PrincipalAPITenant, types.TenantRoleOwner, "mine", 403},
		{types.PrincipalEmbedVisitor, types.TenantRoleOwner, "mine", 403},
		{types.PrincipalWebUser, types.TenantRoleOwner, "other", 404},
	} {
		s := &diagnosticMessageService{}
		h := MessageHandler{MessageService: s}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		ctx := types.WithCaller(context.Background(), types.Caller{UserID: "u", TenantID: 1, Role: tc.role})
		ctx = types.WithPrincipal(ctx, types.Principal{Type: tc.principal, ID: "u"})
		c.Request = httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		c.Params = gin.Params{{Key: "session_id", Value: tc.session}, {Key: "id", Value: "m"}}
		h.GetExecutionDiagnostics(c)
		if w.Code != tc.status {
			t.Fatalf("status=%d wanted=%d body=%s", w.Code, tc.status, w.Body.String())
		}
		if tc.status != 200 && strings.Contains(w.Body.String(), "amount mismatch") {
			t.Fatal("diagnostic leak")
		}
		if tc.status == 403 && s.calls != 0 {
			t.Fatal("unauthorized caller reached storage")
		}
	}
}
