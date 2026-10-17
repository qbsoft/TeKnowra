package einoruntime

import (
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/types"
	"html"
	"strings"
)

// Model-visible IDs are local to this evidence table. The canonical identities
// stay on the server, so a model never chooses between document and chunk UUIDs.
func modelEvidenceJSON(refs []*types.SearchResult) []byte {
	type evidence struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	rows := make([]evidence, len(refs))
	for i, ref := range refs {
		rows[i] = evidence{fmt.Sprintf("E%d", i+1), ref.KnowledgeTitle, ref.Content}
	}
	b, _ := json.Marshal(rows)
	return b
}

func renderModelReview(d Draft, refs []*types.SearchResult) (string, error) {
	aliases := map[string]string{}
	for i, ref := range refs {
		aliases[fmt.Sprintf("E%d", i+1)] = ref.ID
	}
	canonical := Draft{Summary: d.Summary, Citations: make([]string, len(d.Citations))}
	for i, alias := range d.Citations {
		id, ok := aliases[alias]
		if !ok {
			return "", fmt.Errorf("unknown citation")
		}
		canonical.Citations[i] = id
	}
	return RenderValidatedAnswer(canonical, refs)
}

type Draft struct {
	Summary   string   `json:"summary"`
	Citations []string `json:"citations"`
}

func RenderValidatedAnswer(d Draft, refs []*types.SearchResult) (string, error) {
	if strings.TrimSpace(d.Summary) == "" || len(d.Citations) == 0 {
		return "", fmt.Errorf("review requires a nonempty answer and evidence citations")
	}
	lower := strings.ToLower(d.Summary)
	for _, s := range []string{"<kb", "<web", "<search_results"} {
		if strings.Contains(lower, s) {
			return "", fmt.Errorf("model supplied an inline source marker")
		}
	}
	allowed := map[string]*types.SearchResult{}
	for _, r := range refs {
		if r != nil && r.ID != "" && r.KnowledgeID != "" && r.KnowledgeBaseID != "" {
			allowed[r.ID] = r
		}
	}
	var markers []string
	seen := map[string]bool{}
	for _, id := range d.Citations {
		r := allowed[id]
		if r == nil {
			return "", fmt.Errorf("unknown citation")
		}
		if !seen[id] {
			markers = append(markers, fmt.Sprintf(`<kb doc="%s" chunk_id="%s" />`, html.EscapeString(r.KnowledgeTitle), html.EscapeString(id)))
			seen[id] = true
		}
	}
	return d.Summary + "\n\n依据：" + strings.Join(markers, " "), nil
}
