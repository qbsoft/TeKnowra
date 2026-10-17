package service

import (
	"context"
	"errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"testing"
)

type indexPolicyRepoStub struct {
	interfaces.MessageRepository
	disabled bool
	err      error
	linked   int
}

func (r *indexPolicyRepoStub) SessionChatHistoryIndexingDisabled(context.Context, uint64, string) (bool, error) {
	return r.disabled, r.err
}
func (r *indexPolicyRepoStub) UpdateMessageKnowledgeID(context.Context, string, string) error {
	r.linked++
	return nil
}

type indexKnowledgeSpy struct {
	interfaces.KnowledgeService
	calls int
}

func (k *indexKnowledgeSpy) CreateKnowledgeFromPassage(context.Context, string, []string, string) (*types.Knowledge, error) {
	k.calls++
	return &types.Knowledge{ID: "indexed"}, nil
}
func TestContractSessionIndexDoesNotAffectOtherSessions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		disabled bool
		err      error
		want     int
	}{{"contract", true, nil, 0}, {"ordinary", false, nil, 1}, {"lookup-error", false, errors.New("unavailable"), 0}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &indexPolicyRepoStub{disabled: tc.disabled, err: tc.err}
			knowledge := &indexKnowledgeSpy{}
			s := &messageService{messageRepo: repo, knowService: knowledge}
			ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
			ctx = context.WithValue(ctx, types.TenantInfoContextKey, &types.Tenant{ID: 7, ChatHistoryConfig: &types.ChatHistoryConfig{Enabled: true, EmbeddingModelID: "embedding", KnowledgeBaseID: "history"}})
			s.IndexMessageToKB(ctx, "question", "answer", "msg", "session")
			require.Equal(t, tc.want, knowledge.calls)
			require.Equal(t, tc.want, repo.linked)
		})
	}
}
