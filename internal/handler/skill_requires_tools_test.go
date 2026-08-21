package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/skills"
	"github.com/gin-gonic/gin"
)

type stubSkillService struct{ meta []*skills.SkillMetadata }

func (s *stubSkillService) ListPreloadedSkills(context.Context) ([]*skills.SkillMetadata, error) {
	return s.meta, nil
}
func (s *stubSkillService) GetSkillByName(context.Context, string) (*skills.Skill, error) {
	return nil, nil
}

func listSkillsBody(t *testing.T, meta []*skills.SkillMetadata) []map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/skills", nil)

	NewSkillHandler(&stubSkillService{meta: meta}).ListSkills(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	return body.Data
}

// The editor cannot warn about a requirement it never receives. This is the
// wire, so a change to the response shape shows up here rather than as a
// warning that silently stops appearing.
func TestListSkillsSendsRequiresToolsToTheEditor(t *testing.T) {
	data := listSkillsBody(t, []*skills.SkillMetadata{{
		Name: "tyer-contract-review", Description: "审合同",
		RequiresTools: []string{"list_review_templates", "get_review_summary"},
	}})

	if len(data) != 1 {
		t.Fatalf("data = %v, want one skill", data)
	}
	got, ok := data[0]["requires_tools"].([]any)
	if !ok {
		t.Fatalf("requires_tools missing from %v", data[0])
	}
	want := []string{"list_review_templates", "get_review_summary"}
	if len(got) != len(want) {
		t.Fatalf("requires_tools = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("requires_tools[%d] = %v, want %q", i, got[i], w)
		}
	}
}

// omitempty is deliberate: a skill that declares nothing must not arrive as an
// empty list, which the editor would be entitled to read as "needs no tools".
func TestListSkillsOmitsRequiresToolsWhenUndeclared(t *testing.T) {
	data := listSkillsBody(t, []*skills.SkillMetadata{{Name: "plain", Description: "d"}})

	if _, present := data[0]["requires_tools"]; present {
		t.Errorf("requires_tools = %v; a skill declaring nothing should omit the field",
			data[0]["requires_tools"])
	}
}
