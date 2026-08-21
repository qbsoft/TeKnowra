package handler

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
)

// nameIsDegraded reads a name shape rather than re-deriving it, so it is only
// correct while that shape is what MCPTool actually produces. These cases build
// names with the real thing: if upstream changes the naming rule, the assertion
// fails here instead of the warning quietly never firing — or firing on every
// healthy service.
func TestNameIsDegradedMatchesRealToolNames(t *testing.T) {
	cases := []struct {
		serviceName string
		want        bool
		why         string
	}{
		{"mail", false, "an ASCII name keeps its segment"},
		{"CRM-REVIEW-MCP", false, "punctuation is sanitised but letters survive"},
		{"邮件", true, "every character is stripped, so the segment collapses"},
		{"合同审查", true, "same, and it would collide with the one above"},
	}

	for _, c := range cases {
		svc := &types.MCPService{ID: "svc-1", Name: c.serviceName}
		name := tools.NewMCPTool(svc, &types.MCPTool{Name: "send_email"}, nil, nil, 0).Name()

		if got := nameIsDegraded(name); got != c.want {
			t.Errorf("service %q registers its tool as %q; nameIsDegraded = %v, want %v (%s)",
				c.serviceName, name, got, c.want, c.why)
		}
	}
}

// The whole point of the warning: two services that look different to a human
// are indistinguishable to the registry.
func TestDegradedNamesCollideAcrossServices(t *testing.T) {
	a := tools.NewMCPTool(&types.MCPService{ID: "a", Name: "邮件"},
		&types.MCPTool{Name: "send"}, nil, nil, 0).Name()
	b := tools.NewMCPTool(&types.MCPService{ID: "b", Name: "通知"},
		&types.MCPTool{Name: "send"}, nil, nil, 0).Name()

	if a != b {
		t.Skipf("names no longer collide (%q vs %q); the warning may be obsolete", a, b)
	}
	if !nameIsDegraded(a) {
		t.Errorf("two services collide on %q but nameIsDegraded says it is fine", a)
	}
}
