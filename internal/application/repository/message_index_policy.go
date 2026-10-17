package repository

import "context"

// SessionChatHistoryIndexingDisabled includes deleted messages: removing a turn
// must not silently re-enable publication of the remaining confidential context.
func (r *messageRepository) SessionChatHistoryIndexingDisabled(ctx context.Context, tenantID uint64, sessionID string) (bool, error) {
	condition := "m.execution_context->>'chat_history_indexing_disabled' = 'true'"
	if r.db.Dialector.Name() == "sqlite" {
		condition = "json_extract(m.execution_context, '$.chat_history_indexing_disabled') = 1"
	}
	var count int64
	err := r.db.WithContext(ctx).Table("messages AS m").Joins("JOIN sessions s ON s.id = m.session_id").
		Where("s.id = ? AND s.tenant_id = ? AND s.deleted_at IS NULL", sessionID, tenantID).
		Where(condition).Count(&count).Error
	return count > 0, err
}
