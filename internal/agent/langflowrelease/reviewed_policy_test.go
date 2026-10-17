package langflowrelease

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReviewedPolicyVersionsKeepLegacySourcesAndRejectUnknown(t *testing.T) {
	base := Components{Digest: "base", Approved: map[string]string{}}
	legacy, err := WithReviewedExpertComponents(base, "../../../integrations/langflow")
	if err != nil {
		t.Fatal(err)
	}
	current, err := WithReviewedExpertPolicy(base, "../../../integrations/langflow", "v2")
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Digest == current.Digest || strings.Contains(legacy.Approved["ReviewedExpert"], "teknowra_reviewed_experts_v2 import") || !strings.Contains(current.Approved["ReviewedExpert"], "teknowra_reviewed_experts_v2 import") {
		t.Fatal("policy sources or digests crossed")
	}
	if _, err := WithReviewedExpertPolicy(base, "../../../integrations/langflow", "unreviewed"); err == nil {
		t.Fatal("unreviewed policy accepted")
	}
	raw, _ := json.Marshal(map[string]any{"nodes": []any{map[string]any{"data": map[string]any{"node": map[string]any{"template": map[string]any{"_type": "Component", "is_refresh": false, "code": map[string]any{"value": current.Approved["ReviewedExpert"]}}}}}}})
	if GraphReviewedExpertPolicy(raw) != "v2" {
		t.Fatal("new publication lost policy")
	}
}

func TestCanonicalDraftPolicyRetainsEarlierReleaseFingerprints(t *testing.T) {
	base := Components{Digest: "base", Approved: map[string]string{}}
	old, err := WithReviewedExpertPolicy(base, "../../../integrations/langflow", "v2")
	if err != nil {
		t.Fatal(err)
	}
	next, err := WithReviewedExpertPolicy(base, "../../../integrations/langflow", "v3")
	if err != nil {
		t.Fatal(err)
	}
	if old.Digest == next.Digest {
		t.Fatal("changed generator must have a distinct release fingerprint")
	}
	raw, _ := json.Marshal(map[string]any{"nodes": []any{map[string]any{"data": map[string]any{"node": map[string]any{"template": map[string]any{"_type": "Component", "code": map[string]any{"value": next.Approved["ReviewedExpert"]}}}}}}})
	if GraphReviewedExpertPolicy(raw) != "v3" {
		t.Fatal("canonical draft policy not detected")
	}
}
