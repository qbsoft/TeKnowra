package einoruntime

import (
	"github.com/Tencent/WeKnora/internal/types"
	"strings"
	"testing"
)

func TestModelEvidenceUsesRunLocalAliasesAndCanonicalOutput(t *testing.T) {
	refs := []*types.SearchResult{{ID: "actual-chunk-uuid", KnowledgeID: "internal-doc-uuid", KnowledgeBaseID: "internal-kb-uuid", KnowledgeTitle: "合成合同", Content: "金额人民币150000元"}}
	body := string(modelEvidenceJSON(refs))
	for _, secret := range []string{"actual-chunk-uuid", "internal-doc-uuid", "internal-kb-uuid"} {
		if strings.Contains(body, secret) {
			t.Fatal("internal IDs leaked into model evidence")
		}
	}
	if !strings.Contains(body, `"id":"E1"`) {
		t.Fatal(body)
	}
	answer, err := renderModelReview(Draft{Summary: "事实", Citations: []string{"E1"}}, refs)
	if err != nil || !strings.Contains(answer, `chunk_id="actual-chunk-uuid"`) {
		t.Fatalf("%s %v", answer, err)
	}
	if _, err = renderModelReview(Draft{Summary: "事实", Citations: []string{"E2"}}, refs); err == nil {
		t.Fatal("unknown alias accepted")
	}
}
