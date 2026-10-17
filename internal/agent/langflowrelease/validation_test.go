package langflowrelease

import (
	"os"
	"path/filepath"
	"testing"
)

func TestActivationWriteFailureKeepsPreviousVersion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "store.json")
	s, _ := Open(path)
	if err := s.Add(testRelease("v1")); err != nil {
		t.Fatal(err)
	}
	if err := s.Activate("a", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(testRelease("v2")); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	s.path = filepath.Join(blocked, "store.json")
	if s.Activate("a", "v2") == nil {
		t.Fatal("failed persistence accepted")
	}
	active, _ := s.Active("a")
	if active.ID != "v1" {
		t.Fatal("memory activated despite failure")
	}
	restored, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	active, _ = restored.Active("a")
	if active.ID != "v1" {
		t.Fatal("disk activated despite failure")
	}
}
func TestCorruptReleaseStoreIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"releases":{},"active":{"a":"missing"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("corrupt active pointer accepted")
	}
}

func TestReleaseRejectsInvalidFingerprints(t *testing.T) {
	r := testRelease("invalid")
	s, err := Open(t.TempDir() + "/releases.json")
	if err != nil {
		t.Fatal(err)
	}
	r.GraphDigest = "not-a-sha256"
	if s.Add(r) == nil {
		t.Fatal("invalid fingerprint accepted")
	}
}
func TestDigestPreservesNumbersAndNormalizesObjectOrder(t *testing.T) {
	a, err := Digest([]byte(`{"b":2075179812203560962,"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Digest([]byte(`{ "a": 1, "b": 2075179812203560962 }`))
	other, _ := Digest([]byte(`{"b":2075179812203560963,"a":1}`))
	if a != b || a == other {
		t.Fatal("fingerprint changed structure or lost integer precision")
	}
}
