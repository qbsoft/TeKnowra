package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLangflowReleaseRoutesAreRegisteredAndHumanOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	gate := middleware.NewAPIKeyRouteAuthorizer()
	engine.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(types.WithTenantAPIKeyScope(c.Request.Context(), types.TenantAPIKeyScope{FullAccess: true}))
		c.Next()
	})
	engine.Use(gate.Middleware())
	RegisterLangflowRoutes(engine.Group("/api/v1"), handler.NewLangflowHandler(nil, nil), &rbacGuards{})
	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, expected := range []string{"GET /api/v1/agents/:id/langflow/releases", "POST /api/v1/agents/:id/langflow/releases", "POST /api/v1/agents/:id/langflow/releases/:release_id/activate"} {
		if !routes[expected] {
			t.Fatalf("missing route %s", expected)
		}
	}
	for _, row := range []struct{ method, path string }{{http.MethodGet, "/api/v1/agents/a/langflow/releases"}, {http.MethodPost, "/api/v1/agents/a/langflow/releases"}, {http.MethodPost, "/api/v1/agents/a/langflow/releases/r/activate"}} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(row.method, row.path, nil))
		if response.Code != 403 {
			t.Fatalf("%s %s got %d, expected human-only 403", row.method, row.path, response.Code)
		}
	}
}
