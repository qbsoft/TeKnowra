package service

import (
	"encoding/json"
	"testing"
)

// The declaration survives the install only if four links hold: the manifest
// parser reads it, the bundle carries it, the row stores it, and the runtime
// source reads it back. Three of those are cheap to break and none of them
// fails loudly — a lost declaration just means the warning never fires again.
//
// This pins the two links that live in this package.

func TestBundleCarriesRequiresToolsFromManifest(t *testing.T) {
	manifest := `---
name: contract-review
description: 按清单审合同
requires_tools:
  - list_review_templates
  - get_review_checklist
---

正文。
`
	bundle, err := skillBundleFromFiles([]byte("archive"), map[string][]byte{
		"SKILL.md": []byte(manifest),
	})
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	want := []string{"list_review_templates", "get_review_checklist"}
	if len(bundle.RequiresTools) != len(want) {
		t.Fatalf("RequiresTools = %v, want %v", bundle.RequiresTools, want)
	}
	for i, w := range want {
		if bundle.RequiresTools[i] != w {
			t.Errorf("RequiresTools[%d] = %q, want %q", i, bundle.RequiresTools[i], w)
		}
	}
}

// A skill that declares nothing must still install, and must store an empty
// array rather than NULL: the column is NOT NULL, and "declared nothing" is a
// legitimate state, not a defect.
func TestEncodeRequiresToolsHandlesNothingDeclared(t *testing.T) {
	for _, in := range [][]string{nil, {}} {
		got := string(encodeRequiresTools(in))
		if got != "[]" {
			t.Errorf("encodeRequiresTools(%v) = %q, want %q", in, got, "[]")
		}
	}
}

func TestEncodeRequiresToolsRoundTrips(t *testing.T) {
	in := []string{"send_email", "get_review_summary"}
	var out []string
	if err := json.Unmarshal(encodeRequiresTools(in), &out); err != nil {
		t.Fatalf("stored value is not readable JSON: %v", err)
	}
	if len(out) != len(in) || out[0] != in[0] || out[1] != in[1] {
		t.Errorf("round trip = %v, want %v", out, in)
	}
}
