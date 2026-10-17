package service

import (
	"github.com/Tencent/WeKnora/internal/types"
	"strings"
	"testing"
)

func TestLangflowHistoryProjectsCanonicalTextWithoutAttachmentExpansion(t *testing.T) {
	user := &types.Message{Content: "用户短问题", RenderedContent: "old retrieval and attachment body"}
	user.Attachments = types.MessageAttachments{{FileName: "report.pdf", FileType: ".pdf", Content: strings.Repeat("attachment body", 65536)}}
	user.Images = types.MessageImages{{Caption: "previous image body must not be expanded"}}
	assistant := &types.Message{Content: "规范最终答案"}
	messages := buildTextTurnMessages(assistant, []*types.Message{user})
	if len(messages) != 2 || messages[0].Content != "用户短问题" || messages[1].Content != "规范最终答案" {
		t.Fatal("text projection expanded saved prompt", messages)
	}
}
