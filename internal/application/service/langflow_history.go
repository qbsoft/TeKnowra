package service

import (
	"context"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Langflow's text-only contract must be applied before the general loader
// expands images/attachments into Content. Other engines retain their behavior.
func LoadAgentTextHistory(ctx context.Context, repo interfaces.MessageRepository, sessionID string, budget int, retain bool) ([]chat.Message, float64, error) {
	return loadAgentHistory(ctx, repo, sessionID, budget, retain, true)
}
func buildTextTurnMessages(assistant *types.Message, users []*types.Message) []chat.Message {
	messages := make([]chat.Message, 0, len(users)+1)
	for _, user := range users {
		content := user.Content
		if content == "" && (len(user.Images) > 0 || len(user.Attachments) > 0) {
			content = "[上一轮仅上传文件；本轮未载入文件内容]"
		}
		if content != "" {
			messages = append(messages, chat.Message{Role: "user", Content: content})
		}
	}
	if final := finalAnswerHistoryMessage(assistant); final != nil {
		messages = append(messages, chat.Message{Role: "assistant", Content: final.Content})
	}
	return messages
}
