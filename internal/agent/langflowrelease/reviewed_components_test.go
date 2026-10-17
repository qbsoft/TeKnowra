package langflowrelease

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReviewedExpertProfileIsOptInAndPinsCore(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "components", "teknowra"), 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"components/teknowra/reviewed_expert.py": "expert", "components/teknowra/expert_results.py": "merge", "teknowra_reviewed_experts.py": "guard-v1"}
	for name, code := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(code), 0600); err != nil {
			t.Fatal(err)
		}
	}
	base := Components{Digest: "old-profile", Approved: map[string]string{"Agent": "native"}}
	next, err := WithReviewedExpertComponents(base, root)
	if err != nil {
		t.Fatal(err)
	}
	if base.Digest != "old-profile" || len(base.Approved) != 1 || next.Approved["ReviewedExpert"] != "expert" || next.Approved["ExpertResults"] != "merge" {
		t.Fatal("mutated old profile or did not pin new components")
	}
	if err := os.WriteFile(filepath.Join(root, "teknowra_reviewed_experts.py"), []byte("guard-v2"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := WithReviewedExpertComponents(base, root)
	if err != nil || changed.Digest == next.Digest {
		t.Fatal("guard changed without changing fingerprint", err)
	}
}

func TestDetectReviewedGraphDoesNotOptInOldAgents(t *testing.T) {
	for _, raw := range []string{`{"nodes":[]}`, `{"nodes":[{"data":{"type":"Agent"}}]}`} {
		if GraphUsesReviewedExperts([]byte(raw)) {
			t.Fatal("old graph opted in")
		}
	}
	if !GraphUsesReviewedExperts([]byte(`{"nodes":[{"data":{"type":"ReviewedExpert@key"}}]}`)) {
		t.Fatal("new graph missing optional fingerprint")
	}
}
