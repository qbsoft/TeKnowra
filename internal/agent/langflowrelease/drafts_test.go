package langflowrelease

import (
	"path/filepath"
	"testing"
)

func TestManagedDraftSurvivesRestartWithoutChangingActiveRelease(t *testing.T) {
	p := filepath.Join(t.TempDir(), "store.json")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	r := testRelease("v1")
	if err = s.Add(r); err != nil {
		t.Fatal(err)
	}
	if err = s.Activate("a", "v1"); err != nil {
		t.Fatal(err)
	}
	b := r.Binding.Clone()
	b.FlowID = "new-draft"
	if err = s.PutDraft(b); err != nil {
		t.Fatal(err)
	}
	b.Models[0] = "mutated"
	got, ok := s.Draft("a")
	if !ok || got.Models[0] != "m" {
		t.Fatal("draft alias")
	}
	restored, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	got, ok = restored.Draft("a")
	if !ok || got.FlowID != "new-draft" {
		t.Fatal("draft not persisted")
	}
	active, _ := restored.Active("a")
	if active.ID != "v1" || active.Binding.FlowID != "runtime-v1" {
		t.Fatal("draft changed frozen execution")
	}
	b = got
	b.WorkspaceID = 2
	if restored.PutDraft(b) == nil {
		t.Fatal("workspace reassignment accepted")
	}
	if len(restored.Drafts()) != 1 {
		t.Fatal("draft listing lost")
	}
}
