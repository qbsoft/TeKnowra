package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

// agentRunTimeout bounds one scheduled agent run.
//
// Generous, because a job that searches a knowledge base and calls a few tools
// legitimately takes minutes; but bounded, because nobody is watching and a
// wedged run would otherwise hold its claim until the sweeper notices.
const agentRunTimeout = 15 * time.Minute

// agentExecutor runs a job's prompt as a real agent turn.
//
// It reuses the same path as a human conversation (SessionService.AgentQA)
// rather than driving the engine directly. That means skills, tools, MCP
// services, retrieval and the model config all behave exactly as they do when
// the same agent is used interactively — a scheduled run is not a second,
// subtly-different execution mode that drifts away from the real one.
type agentExecutor struct {
	sessions interfaces.SessionService
	messages interfaces.MessageService
	agents   interfaces.CustomAgentService
}

func (e *agentExecutor) Execute(ctx context.Context, job *types.AgentCronJob) (string, error) {
	if job.AgentID == "" {
		return "", errors.New("这个任务没有绑定 agent，无法以 agent 模式执行")
	}

	ctx, cancel := context.WithTimeout(ctx, agentRunTimeout)
	defer cancel()

	agent, err := e.agents.GetAgentByIDAndTenant(ctx, job.AgentID, job.TenantID)
	if err != nil {
		return "", fmt.Errorf("找不到这个任务绑定的 agent：%w", err)
	}
	if agent == nil {
		return "", errors.New("这个任务绑定的 agent 已经不存在了")
	}

	session, err := e.ensureSession(ctx, job)
	if err != nil {
		return "", err
	}

	requestID := uuid.New().String()

	// The prompt is stored as a user message so the run shows up as a normal
	// turn in the session. Without it the assistant reply would hang under a
	// question nobody asked.
	userMsg, err := e.messages.CreateMessage(ctx, &types.Message{
		SessionID:   session.ID,
		Role:        "user",
		Content:     job.Prompt,
		RequestID:   requestID,
		CreatedAt:   time.Now(),
		IsCompleted: true,
		Channel:     "cron",
	})
	if err != nil {
		return "", fmt.Errorf("创建任务消息失败：%w", err)
	}

	assistantMsg, err := e.messages.CreateMessage(ctx, &types.Message{
		SessionID:   session.ID,
		Role:        "assistant",
		RequestID:   requestID,
		CreatedAt:   time.Now(),
		IsCompleted: false,
		Channel:     "cron",
	})
	if err != nil {
		return "", fmt.Errorf("创建回复占位失败：%w", err)
	}

	answer, err := e.runAndCollect(ctx, session, agent, job.Prompt, assistantMsg.ID, userMsg.ID)
	if err != nil {
		return answer, err
	}
	if strings.TrimSpace(answer) == "" {
		// Not an error: an agent that finds nothing worth saying is the
		// watchdog case, same as an empty HTTP body.
		return "", nil
	}
	return answer, nil
}

// ensureSession returns the job's own conversation, creating it on first run.
//
// One session per job, not one per run. A run every five minutes would
// otherwise leave 288 sessions a day behind, and the user would have no single
// place to read what the job has been saying. Reusing it means the job's whole
// history is one readable thread.
//
// Whether a run can SEE that history is the agent's own MultiTurnEnabled
// setting, deliberately not overridden here: a scheduled run should behave
// like the same agent does interactively.
func (e *agentExecutor) ensureSession(ctx context.Context, job *types.AgentCronJob) (*types.Session, error) {
	// The job id doubles as the session id — both are UUIDs, and it makes the
	// mapping obvious without another column to keep in sync.
	if existing, err := e.sessions.GetSessionByID(ctx, job.TenantID, job.ID); err == nil && existing != nil {
		return existing, nil
	}

	title := job.Name
	if title == "" {
		title = "定时任务"
	}
	session, err := e.sessions.CreateSession(ctx, &types.Session{
		ID:          job.ID,
		Title:       title,
		Description: "定时任务的执行记录",
		TenantID:    job.TenantID,
		UserID:      job.CreatorUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("创建任务会话失败：%w", err)
	}
	return session, nil
}

// runAndCollect performs the turn and returns the assistant's final answer.
func (e *agentExecutor) runAndCollect(
	ctx context.Context,
	session *types.Session,
	agent *types.CustomAgent,
	query, assistantMessageID, userMessageID string,
) (string, error) {
	var (
		mu     sync.Mutex
		buf    strings.Builder
		bufErr string
	)

	eventBus := event.NewEventBus()

	// The final answer arrives in chunks; accumulate rather than keeping the
	// last one.
	eventBus.On(event.EventAgentFinalAnswer, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentFinalAnswerData)
		if !ok {
			if ptr, okPtr := evt.Data.(*event.AgentFinalAnswerData); okPtr {
				data = *ptr
			} else {
				return nil
			}
		}
		mu.Lock()
		buf.WriteString(data.Content)
		mu.Unlock()
		return nil
	})

	// Errors surface as events rather than as a return value on some paths, so
	// capture them or a failed run would be recorded as an empty success.
	eventBus.On(event.EventError, func(_ context.Context, evt event.Event) error {
		mu.Lock()
		if bufErr == "" {
			bufErr = fmt.Sprintf("%v", evt.Data)
		}
		mu.Unlock()
		return nil
	})

	req := &types.QARequest{
		Session:            session,
		Query:              query,
		AssistantMessageID: assistantMessageID,
		UserMessageID:      userMessageID,
		CustomAgent:        agent,
	}

	runErr := e.sessions.AgentQA(ctx, req, eventBus)

	mu.Lock()
	answer := buf.String()
	eventErr := bufErr
	mu.Unlock()

	if runErr != nil {
		return answer, runErr
	}
	if eventErr != "" {
		return answer, errors.New(eventErr)
	}
	return answer, nil
}

// newAgentExecutor wires the agent execution mode, or reports why it cannot be
// wired so the caller can degrade instead of panicking at 3am.
func newAgentExecutor(
	sessions interfaces.SessionService,
	messages interfaces.MessageService,
	agents interfaces.CustomAgentService,
) (cronExecutor, error) {
	if sessions == nil || messages == nil || agents == nil {
		return nil, errors.New("agent execution requires session, message and agent services")
	}
	return &agentExecutor{sessions: sessions, messages: messages, agents: agents}, nil
}

// WithAgentExecution enables the agent mode on a runner.
//
// Kept separate from the constructor so that a deployment which only wants
// scripted jobs — or one where these services are unavailable — still gets a
// working runner rather than a nil dependency waiting to panic.
func (r *AgentCronRunner) WithAgentExecution(
	sessions interfaces.SessionService,
	messages interfaces.MessageService,
	agents interfaces.CustomAgentService,
) *AgentCronRunner {
	exec, err := newAgentExecutor(sessions, messages, agents)
	if err != nil {
		logger.Warnf(context.Background(), "[AgentCron] agent execution unavailable: %v", err)
		return r
	}
	r.executors[types.CronModeAgent] = exec
	return r
}
