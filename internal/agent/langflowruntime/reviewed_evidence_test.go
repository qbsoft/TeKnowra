package langflowruntime

import (
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestReviewedEvidenceRejectsSameCountChangedSource(t *testing.T) {
	original := []*types.SearchResult{{ID: "s1", Content: "总价100000元"}}
	same := []*types.SearchResult{{ID: "s1", Content: "总价100000元"}}
	changed := []*types.SearchResult{{ID: "s1", Content: "总价200000元"}}
	if !publicationEvidenceMatches(original, same, true) {
		t.Fatal("unchanged evidence rejected")
	}
	if publicationEvidenceMatches(original, changed, true) {
		t.Fatal("changed facts accepted merely because count matched")
	}
	if publicationEvidenceMatches(original, nil, true) {
		t.Fatal("revoked evidence accepted")
	}
}
