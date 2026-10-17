package session

import (
	"context"
	"errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

type contractDocumentStub struct {
	interfaces.TemporaryDocumentService
	docs map[string]*types.TemporaryDocument
}

type contractAsyncDocumentStub struct {
	interfaces.TemporaryDocumentService
	doc              *types.TemporaryDocument
	reads            int
	readyStatus      string
	foreignWhenReady bool
	alwaysPending    bool
}

func (s *contractAsyncDocumentStub) Get(context.Context, uint64, string, string) (*types.TemporaryDocument, error) {
	s.reads++
	doc := *s.doc
	if s.alwaysPending || s.reads < 3 {
		doc.Status = types.TemporaryDocumentStatusProcessing
	} else {
		doc.Status = s.readyStatus
		if s.foreignWhenReady {
			doc.SessionID = "other"
		}
	}
	return &doc, nil
}

func TestContractAttachmentWaitsForAsyncParsing(t *testing.T) {
	h, req, doc := contractFixture()
	h.temporaryDocuments = &contractAsyncDocumentStub{doc: doc, readyStatus: types.TemporaryDocumentStatusReady}
	require.NoError(t, h.prepareSessionContractMaterials(context.Background(), req))
	require.Equal(t, doc.Content, req.attachments[0].Content)
}

func TestContractAttachmentAsyncWaitKeepsFailureAndScopeGuards(t *testing.T) {
	for _, name := range []string{"cancelled", "failed", "foreign-after-wait"} {
		t.Run(name, func(t *testing.T) {
			h, req, doc := contractFixture()
			stub := &contractAsyncDocumentStub{doc: doc, readyStatus: types.TemporaryDocumentStatusReady}
			ctx := context.Background()
			if name == "cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 5*time.Millisecond)
				defer cancel()
				stub.alwaysPending = true
			}
			if name == "failed" {
				stub.readyStatus = types.TemporaryDocumentStatusFailed
			}
			stub.foreignWhenReady = name == "foreign-after-wait"
			h.temporaryDocuments = stub
			require.Error(t, h.prepareSessionContractMaterials(ctx, req))
			require.Empty(t, req.attachments)
		})
	}
}

func (s *contractDocumentStub) Get(_ context.Context, _ uint64, _ string, id string) (*types.TemporaryDocument, error) {
	return s.docs[id], nil
}

type contractInventoryStub struct {
	interfaces.MessageService
	attachments types.MessageAttachments
	err         error
}

func (s *contractInventoryStub) GetSessionAttachments(context.Context, string) (types.MessageAttachments, error) {
	return s.attachments, s.err
}

func contractFixture() (*Handler, *qaRequestContext, *types.TemporaryDocument) {
	doc := &types.TemporaryDocument{ID: "doc", TenantID: 7, SessionID: "session", FileName: "contract.txt", FileType: ".txt", Content: "合同总价100000元，预付款40%。", Status: types.TemporaryDocumentStatusReady, TokenCount: 30, ExpiresAt: time.Now().Add(time.Hour)}
	inv := &contractInventoryStub{attachments: types.MessageAttachments{{ID: "doc", FileName: doc.FileName}}}
	h := &Handler{temporaryDocuments: &contractDocumentStub{docs: map[string]*types.TemporaryDocument{"doc": doc}}, messageService: inv}
	req := &qaRequestContext{sessionID: "session", session: &types.Session{ID: "session", TenantID: 7}, customAgent: &types.CustomAgent{Config: types.CustomAgentConfig{AllowSessionAttachments: true}}, query: "审查合同", attachmentIDs: []string{"doc"}}
	return h, req, doc
}
func TestContractAttachmentFullTextAndFollowup(t *testing.T) {
	h, r, d := contractFixture()
	require.NoError(t, h.prepareSessionContractMaterials(context.Background(), r))
	require.Equal(t, d.Content, r.attachments[0].Content)
	require.Equal(t, "full", r.attachments[0].ContentMode)
	r.attachmentIDs = nil
	r.attachments = nil
	r.query = "再看预付款"
	require.NoError(t, h.prepareSessionContractMaterials(context.Background(), r))
	require.Contains(t, r.attachments[0].Content, "40%")
}
func TestContractAttachmentRejectsUnavailableOrIncomplete(t *testing.T) {
	for _, name := range []string{"expired", "foreign-session", "foreign-tenant", "processing", "failed", "empty", "too-large", "oversized-bytes", "image"} {
		t.Run(name, func(t *testing.T) {
			h, r, d := contractFixture()
			switch name {
			case "expired":
				d.ExpiresAt = time.Now().Add(-time.Second)
			case "foreign-session":
				d.SessionID = "other"
			case "foreign-tenant":
				d.TenantID = 8
			case "processing":
				d.Status = types.TemporaryDocumentStatusProcessing
			case "failed":
				d.Status = types.TemporaryDocumentStatusFailed
			case "empty":
				d.Content = ""
			case "too-large":
				d.TokenCount = 12001
			case "oversized-bytes":
				d.Content = strings.Repeat("x", 100000)
			case "image":
				d.FileType = ".png"
			}
			ctx := context.Background()
			if name == "processing" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 5*time.Millisecond)
				defer cancel()
			}
			require.Error(t, h.prepareSessionContractMaterials(ctx, r))
		})
	}
}
func TestContractAttachmentSelectionAndInventoryFailure(t *testing.T) {
	h, r, _ := contractFixture()
	r.attachmentIDs = nil
	inv := h.messageService.(*contractInventoryStub)
	inv.attachments = append(inv.attachments, types.MessageAttachment{ID: "other"})
	require.Error(t, h.prepareSessionContractMaterials(context.Background(), r))
	inv.err = errors.New("unavailable")
	require.Error(t, h.prepareSessionContractMaterials(context.Background(), r))
}

func TestContractAttachmentMissingOCRAndDuplicateReferences(t *testing.T) {
	h, r, d := contractFixture()
	r.attachmentIDs = []string{"missing"}
	require.ErrorContains(t, h.prepareSessionContractMaterials(context.Background(), r), "不属于本会话")
	r.attachmentIDs = []string{"doc"}
	d.Metadata = types.JSON(`{"image_understanding":"vlm"}`)
	require.ErrorContains(t, h.prepareSessionContractMaterials(context.Background(), r), "OCR")
	d.Metadata = nil
	r.attachmentIDs = nil
	h.messageService.(*contractInventoryStub).attachments = types.MessageAttachments{{ID: "doc"}, {ID: "doc"}}
	require.NoError(t, h.prepareSessionContractMaterials(context.Background(), r))
	require.Len(t, r.attachments, 1)
}

type contractPersistStub struct {
	interfaces.MessageService
	disabled  bool
	lookupErr error
	rows      []*types.Message
}

func (s *contractPersistStub) SessionChatHistoryIndexingDisabled(context.Context, uint64, string) (bool, error) {
	return s.disabled, s.lookupErr
}
func (s *contractPersistStub) CreateMessage(_ context.Context, m *types.Message) (*types.Message, error) {
	m.ID = m.Role
	s.rows = append(s.rows, m)
	return m, nil
}
func TestContractSessionPolicySurvivesAgentSwitch(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		store := &contractPersistStub{disabled: disabled}
		h := &Handler{messageService: store}
		r := &qaRequestContext{sessionID: "s", session: &types.Session{TenantID: 7}, customAgent: &types.CustomAgent{}}
		require.NoError(t, h.persistTurnMessages(context.Background(), r))
		require.Equal(t, disabled, r.assistantMessage.ExecutionContext.ChatHistoryIndexingDisabled)
	}
	store := &contractPersistStub{lookupErr: errors.New("db unavailable")}
	h := &Handler{messageService: store}
	r := &qaRequestContext{sessionID: "s", session: &types.Session{TenantID: 7}}
	require.Error(t, h.persistTurnMessages(context.Background(), r))
	require.Empty(t, store.rows)
}
func TestContractIndexSnapshotOnlyOptsOutExplicitAgents(t *testing.T) {
	for _, tc := range []struct{ disable, attachments, want bool }{{false, false, false}, {true, false, true}, {false, true, true}} {
		a := &types.CustomAgent{Config: types.CustomAgentConfig{DisableChatHistoryIndexing: tc.disable, AllowSessionAttachments: tc.attachments}}
		snapshot, _, _, _ := buildMessageExecutionContext(context.Background(), a, 0, "", nil, nil, nil, nil, nil, nil, false)
		require.Equal(t, tc.want, snapshot.ChatHistoryIndexingDisabled)
	}
}
