package session

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type unavailableSessionStub struct {
	interfaces.SessionService
	ownerErr        error
	holdErr         error
	holds, releases int
}

func (s *unavailableSessionStub) GetOwnedSession(_ context.Context, id string) (*types.Session, error) {
	return &types.Session{ID: id}, s.ownerErr
}
func (s *unavailableSessionStub) HoldSandboxTurn(context.Context, string, string) (func(), error) {
	s.holds++
	if s.holdErr != nil {
		return nil, s.holdErr
	}
	return func() { s.releases++ }, nil
}

type unavailableMessageStub struct {
	steerPersistingMessageStub
	failAssistant bool
}

func (s *unavailableMessageStub) CreateMessage(ctx context.Context, msg *types.Message) (*types.Message, error) {
	if msg.Role == "assistant" && s.failAssistant {
		return nil, errors.New("private database detail")
	}
	return s.steerPersistingMessageStub.CreateMessage(ctx, msg)
}

func TestSharedAgentUnavailablePersistsOnlyOwnedRejectedTurn(t *testing.T) {
	for _, tc := range []struct {
		name         string
		ownerErr     error
		live         string
		fail         bool
		holdErr      error
		status, rows int
	}{
		{"revoked", nil, "", false, nil, 404, 2},
		{"not owner", errors.New("not owner"), "", false, nil, 404, 0},
		{"running", nil, "live-assistant", false, nil, 409, 0},
		{"database unavailable", nil, "", true, nil, 503, 0},
		{"send lease busy", nil, "", false, sandbox.ErrSessionTurnActive, 409, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			sessions := &unavailableSessionStub{ownerErr: tc.ownerErr, holdErr: tc.holdErr}
			messages := &unavailableMessageStub{failAssistant: tc.fail}
			h := &Handler{sessionService: sessions, messageService: messages, streamManager: &steerLiveRunLookupStub{assistantID: tc.live}, agentShareService: &resolveAgentShareStub{}}
			r := gin.New()
			r.Use(middleware.ErrorHandler())
			r.Use(func(c *gin.Context) {
				c.Set(types.UserIDContextKey.String(), "sales01")
				c.Set(types.TenantIDContextKey.String(), uint64(7))
			})
			r.POST("/agent-chat/:session_id", h.AgentQA)
			request := httptest.NewRequest(http.MethodPost, "/agent-chat/owned-session", bytes.NewBufferString(`{"query":"复核合同","agent_id":"revoked-agent","agent_source_tenant_id":84,"knowledge_base_ids":["forbidden-kb"],"attachment_ids":["forbidden-file"]}`))
			request.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, request)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.Len(t, messages.byID, tc.rows)
			if tc.holdErr == nil {
				require.Equal(t, sessions.holds, sessions.releases)
			} else {
				require.Zero(t, sessions.releases)
			}
			if tc.rows == 2 {
				require.Contains(t, w.Body.String(), `"persisted_failure":true`)
				for _, msg := range messages.byID {
					require.Equal(t, "owned-session", msg.SessionID)
					require.Empty(t, msg.Attachments)
					if msg.Role == "assistant" {
						require.True(t, msg.IsCompleted)
						require.Contains(t, msg.Content, "共享智能体")
						require.Contains(t, msg.Content, "撤销")
					}
				}
			} else {
				require.NotContains(t, w.Body.String(), "persisted_failure")
				require.NotContains(t, w.Body.String(), "private database detail")
			}
		})
	}
}
