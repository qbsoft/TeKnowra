package repository

import (
	"context"
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"strings"
	"testing"
)

func TestExecutionDiagnosticRepositoryRoundTripDoesNotRewriteHistory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)
	if err = db.AutoMigrate(&types.Message{}, &types.MessageArtifactRecord{}); err != nil {
		t.Fatal(err)
	}
	repo := NewMessageRepository(db)
	ctx := context.Background()
	old := &types.Message{ID: "old", SessionID: "s", Role: "assistant", Content: "historical answer", IsCompleted: true, AgentSteps: types.AgentSteps{{DefinitionVersion: "contract-review-v1"}}}
	if _, err = repo.CreateMessage(ctx, old); err != nil {
		t.Fatal(err)
	}
	current := &types.Message{ID: "new", SessionID: "s", Role: "assistant"}
	if _, err = repo.CreateMessage(ctx, current); err != nil {
		t.Fatal(err)
	}
	current.IsCompleted = true
	current.Content = "task failed"
	current.AgentSteps = types.AgentSteps{{DefinitionVersion: "contract-review-v1", ImplementationVersion: "source-immutable-hash"}}
	current.ExecutionContext.ExecutionDiagnostics = []types.ExecutionDiagnostic{types.NewExecutionDiagnostic("eino", "validate", "fact_audit_rejected", []string{"private-review-reason"})}
	if err = repo.UpdateMessage(ctx, current); err != nil {
		t.Fatal(err)
	}
	restored, err := repo.GetMessage(ctx, "s", current.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.ExecutionContext.ExecutionDiagnostics) != 1 || restored.ExecutionContext.ExecutionDiagnostics[0].Reasons[0] != "private-review-reason" || restored.AgentSteps[0].ImplementationVersion != "source-immutable-hash" {
		t.Fatal("repository lost diagnostic or implementation")
	}
	raw, _ := json.Marshal(restored)
	if strings.Contains(string(raw), "private-review-reason") {
		t.Fatal("ordinary message API would disclose private diagnostics")
	}
	restoredOld, err := repo.GetMessage(ctx, "s", old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restoredOld.Content != "historical answer" || restoredOld.AgentSteps[0].ImplementationVersion != "" || len(restoredOld.ExecutionContext.ExecutionDiagnostics) != 0 {
		t.Fatal("historical message was rewritten")
	}
}
