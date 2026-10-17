package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestEinoRetirementRejectsBeforeResourcesOrFallback(t *testing.T) {
	t.Setenv("TEKNOWRA_EINO_DISABLED", "true")
	// No config/models/resources: retirement must reject before touching them.
	s := &agentService{}
	engine, err := s.createEinoEngine(context.Background(), &types.AgentConfig{}, nil, nil, nil, "historical-session")
	if engine != nil || err == nil || !strings.Contains(err.Error(), "Eino执行已停用") {
		t.Fatalf("expected explicit retirement error, got %v / %v", engine, err)
	}
}
