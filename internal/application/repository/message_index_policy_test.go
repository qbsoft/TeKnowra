package repository

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSessionHistoryIndexPolicyIsStickyAndScoped(t *testing.T) {
	db := newArtifactTestDB(t)
	contract := createSession(t, db, &types.Session{TenantID: 7})
	normal := createSession(t, db, &types.Session{TenantID: 7})
	repo := &messageRepository{db: db}
	message, err := repo.CreateMessage(context.Background(), &types.Message{SessionID: contract, Role: "assistant", ExecutionContext: types.MessageExecutionContext{ChatHistoryIndexingDisabled: true}})
	require.NoError(t, err)
	disabled, err := repo.SessionChatHistoryIndexingDisabled(context.Background(), 7, contract)
	require.NoError(t, err)
	require.True(t, disabled)
	disabled, err = repo.SessionChatHistoryIndexingDisabled(context.Background(), 7, normal)
	require.NoError(t, err)
	require.False(t, disabled)
	disabled, err = repo.SessionChatHistoryIndexingDisabled(context.Background(), 8, contract)
	require.NoError(t, err)
	require.False(t, disabled)
	// A deleted sensitive turn must not re-enable automatic indexing in its session.
	require.NoError(t, db.Delete(&types.Message{}, "id = ?", message.ID).Error)
	disabled, err = repo.SessionChatHistoryIndexingDisabled(context.Background(), 7, contract)
	require.NoError(t, err)
	require.True(t, disabled)
}
