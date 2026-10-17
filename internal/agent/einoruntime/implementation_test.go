package einoruntime

import (
	"strings"
	"testing"
)

func TestImplementationFingerprintIsBoundToPublishedDefinition(t *testing.T) {
	a := ImplementationVersion("contract-review-v1")
	if !strings.HasPrefix(a, "eino-sha256:") || len(a) != 76 || a != ImplementationVersion("contract-review-v1") {
		t.Fatal("unstable fingerprint")
	}
	if a == ImplementationVersion("knowledge-summary-v1") {
		t.Fatal("definition not included")
	}
	if ImplementationVersion("unpublished") == "" {
		return
	}
	t.Fatal("unpublished implementation fingerprint accepted")
}
