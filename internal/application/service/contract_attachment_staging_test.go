package service

import (
	"context"
	"errors"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"testing"
)

type rejectedAttachmentInventory struct {
	interfaces.MessageRepository
	calls int
}

func (r *rejectedAttachmentInventory) GetSessionAttachments(context.Context, string) (types.MessageAttachments, error) {
	r.calls++
	return nil, errors.New("inventory contains expired, oversized and parent-session files")
}
func TestContractTextProjectionDoesNotStageWholeInventory(t *testing.T) {
	repo := &rejectedAttachmentInventory{}
	stageCalled := false
	stage := func(types.MessageAttachments) ([]stagedSessionAttachment, error) { stageCalled = true; return nil, nil }
	rows, err := stageAttachmentInventory(context.Background(), repo, "current", true, stage)
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Zero(t, repo.calls)
	require.False(t, stageCalled, "text projection must not reconcile or remove historical sandbox files")
	_, err = stageAttachmentInventory(context.Background(), repo, "current", false, stage)
	require.Error(t, err)
	require.Equal(t, 1, repo.calls)
}
func TestContractTextProjectionComposesReadOnlySkillsButRequiresScriptPolicy(t *testing.T) {
	no := false
	b := langflowdef.Binding{AllowSessionAttachments: true, Skills: []string{"file-reader"}, SandboxConfigID: "sandbox"}
	cfg := &types.AgentConfig{MemoryEnabled: &no, AllowSessionAttachments: true, SkillsEnabled: true, AllowedSkills: []string{"file-reader"}, SandboxConfigID: "sandbox"}
	require.NoError(t, validateLangflowCapabilities(b, cfg))
	b.AllowSkillScripts = true
	require.Error(t, validateLangflowCapabilities(b, cfg))
}
