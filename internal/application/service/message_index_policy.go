package service

import (
	"context"
	"errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

func (s *messageService) SessionChatHistoryIndexingDisabled(ctx context.Context, tenantID uint64, sessionID string) (bool, error) {
	policy, ok := s.messageRepo.(interfaces.SessionChatHistoryPolicy)
	if !ok {
		return true, errors.New("session indexing policy unavailable")
	}
	return policy.SessionChatHistoryIndexingDisabled(ctx, tenantID, sessionID)
}

// GetSessionAttachments authorizes the session read before projecting attachments.
func (s *messageService) GetSessionAttachments(ctx context.Context, sessionID string) (types.MessageAttachments, error) {
	if _, err := s.GetRecentMessagesBySession(ctx, sessionID, 1); err != nil {
		return nil, err
	}
	return s.messageRepo.GetSessionAttachments(ctx, sessionID)
}
