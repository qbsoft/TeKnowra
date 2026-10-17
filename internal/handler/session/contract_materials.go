package session

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"strings"
	"time"
)

// prepareSessionContractMaterials reopens session-owned temporary originals on
// every turn. Never reuse selected chunks or stale text embedded in old messages.
func (h *Handler) prepareSessionContractMaterials(ctx context.Context, r *qaRequestContext) error {
	if r.customAgent == nil || !r.customAgent.Config.AllowSessionAttachments {
		return nil
	}
	fail := func(reason string) error { return fmt.Errorf("会话附件：%s", reason) }
	if r.session == nil || h.temporaryDocuments == nil {
		return fail("材料服务不可用，请稍后重试")
	}
	ids := dedupRequestStrings(r.attachmentIDs)
	if len(ids) == 0 {
		inventory, ok := h.messageService.(interfaces.SessionAttachmentInventory)
		if !ok {
			return fail("无法核对会话材料，请重新选择附件")
		}
		rows, err := inventory.GetSessionAttachments(ctx, r.sessionID)
		if err != nil {
			return fail("无法核对会话材料，请稍后重试")
		}
		for _, row := range rows {
			if row.ID != "" {
				ids = append(ids, row.ID)
			}
		}
		ids = dedupRequestStrings(ids)
		if len(ids) > 1 {
			return fail("本会话有多份材料，请在本轮明确选择需要审查的附件")
		}
		if len(ids) == 0 && len(r.attachments) > 0 {
			return fail("请使用会话附件上传后重试")
		}
	}
	if len(ids) > types.MaxTemporaryAttachmentsPerMessage {
		return fail("一次最多选择5份材料")
	}
	var materials types.MessageAttachments
	tokens, size := 0, 0
	waitTimeout := attachmentParseWaitTimeout()
	if seconds := r.customAgent.Config.AttachmentParseWaitTimeoutSec; seconds > 0 {
		waitTimeout = time.Duration(seconds) * time.Second
	}
	parseDeadline := time.Now().Add(waitTimeout)
	for _, id := range ids {
		doc, err := h.temporaryDocuments.Get(ctx, r.session.TenantID, r.sessionID, id)
		if err != nil || doc == nil || doc.TenantID != r.session.TenantID || doc.SessionID != r.sessionID || !doc.ExpiresAt.After(time.Now()) {
			return fail("材料已过期、被删除或不属于本会话，请重新上传")
		}
		if doc.Status == types.TemporaryDocumentStatusUploaded || doc.Status == types.TemporaryDocumentStatusProcessing {
			// Upstream uploads now return before parsing. Reuse its bounded,
			// cancellable wait, then reopen and revalidate the complete original.
			h.waitForAttachments(ctx, r.session.TenantID, r.sessionID, []string{id}, time.Until(parseDeadline))
			if err := ctx.Err(); err != nil {
				return err
			}
			doc, err = h.temporaryDocuments.Get(ctx, r.session.TenantID, r.sessionID, id)
			if err != nil || doc == nil || doc.TenantID != r.session.TenantID || doc.SessionID != r.sessionID || !doc.ExpiresAt.After(time.Now()) {
				return fail("材料已过期、被删除或不属于本会话，请重新上传")
			}
		}
		if doc.Status != types.TemporaryDocumentStatusReady {
			return fail("材料尚未解析完成或解析失败，请检查附件状态后重试")
		}
		ext := strings.TrimPrefix(strings.ToLower(doc.FileType), ".")
		switch ext {
		case "txt", "md", "pdf", "docx", "doc":
		default:
			return fail("当前仅支持TXT、Markdown、PDF及Word文本材料")
		}
		if len(r.customAgent.Config.SupportedFileTypes) > 0 && !containsFileType(r.customAgent.Config.SupportedFileTypes, ext) {
			return fail("文件类型不在当前智能体允许范围")
		}
		var meta map[string]interface{}
		if len(doc.Metadata) > 0 && json.Unmarshal(doc.Metadata, &meta) != nil {
			return fail("无法确认材料解析完整性，请重新上传文本版本")
		}
		if meta["image_understanding"] != nil {
			return fail("扫描件OCR尚不能确认覆盖全部页面，请上传可提取文本的合同")
		}
		if strings.TrimSpace(doc.Content) == "" || doc.TokenCount <= 0 {
			return fail("材料没有可用正文，请上传可提取文本的合同")
		}
		tokens += doc.TokenCount
		size += len(doc.Content)
		if tokens > 12000 || size > 64*1024 {
			return fail("材料超过完整审查输入上限（约12000 token、64KiB正文），请拆分后明确审查范围")
		}
		materials = append(materials, types.MessageAttachment{ID: doc.ID, FileName: doc.FileName, FileType: doc.FileType, FileSize: doc.FileSize, Content: doc.Content, ContentMode: "full", TokenCount: doc.TokenCount, SelectedChunks: doc.ChunkCount, TotalChunks: doc.ChunkCount})
	}
	r.attachments = materials
	return nil
}
