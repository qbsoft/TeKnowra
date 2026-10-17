package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLangflowCredentialsRejectEmptyAndSharedIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	for _, raw := range []string{`{}`, `{"editor_api_key":"same","runtime_api_key":"same","runtime_owner_id":"runner"}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadLangflowCredentials(path); err == nil {
			t.Fatal("invalid or shared service credentials accepted")
		}
	}
	if err := os.WriteFile(path, []byte(`{"editor_api_key":"editor-key","runtime_api_key":"runner-key","runtime_owner_id":"runner"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLangflowCredentials(path); err != nil {
		t.Fatal(err)
	}
}
