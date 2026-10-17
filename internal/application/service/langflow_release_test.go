package service

import (
	"github.com/Tencent/WeKnora/internal/config"
	"testing"
)

func TestDisabledLangflowReleaseServiceDoesNotRequireCredentials(t *testing.T) {
	s, err := NewLangflowReleaseService(&config.Config{}, nil)
	if err != nil || s.Enabled() {
		t.Fatalf("legacy startup rejected: %v", err)
	}
}
