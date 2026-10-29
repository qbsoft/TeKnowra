package session

import (
	"context"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

const unavailableSharedAgentMessage = "任务未完成：共享智能体已不可用或授权已撤销，请联系管理员确认共享后重试。"

// Called only after GetOwnedSession succeeds. Preserve the rejected request in
// the caller's history without resolving any source resources or starting QA.
func (h *Handler) persistUnavailableSharedAgent(ctx context.Context, c *gin.Context, session *types.Session, query, requestID string) error {
	req := &qaRequestContext{
		ctx: ctx, c: c, session: session, sessionID: session.ID,
		userInput: query, requestID: requestID,
		assistantMessage: &types.Message{
			SessionID: session.ID, Role: "assistant", RequestID: requestID,
			Content: unavailableSharedAgentMessage, IsCompleted: true,
		},
	}
	if err := h.rejectIfOtherAgentRunLive(ctx, req); err != nil {
		return err
	}
	if err := h.rejectIfSessionRewinding(ctx, session.ID); err != nil {
		return err
	}
	ctx, release, held := h.holdQATurn(ctx, req)
	if !held {
		if last := c.Errors.Last(); last != nil {
			return last.Err
		}
		return errors.NewServiceUnavailableError("Failed to hold rejected turn")
	}
	defer release()
	// A run can become live between the first check and acquiring the lease.
	if err := h.rejectIfOtherAgentRunLive(ctx, req); err != nil {
		return err
	}
	if err := h.persistTurnMessages(ctx, req); err != nil {
		logger.Warnf(ctx, "Failed to persist unavailable shared-agent turn for session %s", session.ID)
		return errors.NewServiceUnavailableError("Failed to save rejected turn; please retry")
	}
	return errors.NewNotFoundError("Shared agent not found").WithDetails(map[string]any{
		"persisted_failure": true, "session_id": session.ID,
		"assistant_message_id": req.assistantMessage.ID,
	})
}
