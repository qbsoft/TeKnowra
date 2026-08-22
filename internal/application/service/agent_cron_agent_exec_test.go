package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

// The three service interfaces are large and this executor touches four
// methods between them, so the fakes embed the interface and override only
// what is used. Anything else being called would nil-panic, which is the
// behaviour we want from a test double.

type fakeSessions struct {
	interfaces.SessionService

	existing *types.Session
	created  []*types.Session
	// qa stands in for the agent turn: it receives the bus and emits whatever
	// the test wants the run to produce.
	qa func(ctx context.Context, req *types.QARequest, bus *event.EventBus) error

	lastReq *types.QARequest
}

func (f *fakeSessions) GetSessionByID(_ context.Context, _ uint64, _ string) (*types.Session, error) {
	if f.existing == nil {
		return nil, errors.New("not found")
	}
	return f.existing, nil
}

func (f *fakeSessions) CreateSession(_ context.Context, s *types.Session) (*types.Session, error) {
	// types.Session.BeforeCreate overwrites whatever id it is handed. Mirroring
	// that here is the point of this fake: handing the caller's own id back
	// makes the job's id look like it survives into the session, and believing
	// that is what left one session behind per run.
	stored := *s
	stored.ID = fmt.Sprintf("session-generated-%d", len(f.created)+1)
	f.created = append(f.created, &stored)
	f.existing = &stored
	return &stored, nil
}

func (f *fakeSessions) AgentQA(ctx context.Context, req *types.QARequest, bus *event.EventBus) error {
	f.lastReq = req
	if f.qa == nil {
		return nil
	}
	return f.qa(ctx, req, bus)
}

type fakeMessages struct {
	interfaces.MessageService
	created []*types.Message
	// updated holds the messages finishMessage closed out. A run that never
	// updates its assistant message leaves it stuck at "in progress", and the
	// front end reports a broken stream when the session is opened.
	updated []*types.Message
}

func (f *fakeMessages) CreateMessage(_ context.Context, m *types.Message) (*types.Message, error) {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	// Store a copy, not the caller's pointer. A real repository hands back a
	// row, so what was written stays written; keeping the pointer means
	// finishMessage's later mutation rewrites history, and an assertion about
	// what the placeholder looked like silently becomes an assertion about the
	// finished message.
	stored := *m
	f.created = append(f.created, &stored)
	return m, nil
}

func (f *fakeMessages) UpdateMessage(_ context.Context, m *types.Message) error {
	f.updated = append(f.updated, m)
	return nil
}

type fakeAgents struct {
	interfaces.CustomAgentService
	agent *types.CustomAgent
	err   error
}

func (f *fakeAgents) GetAgentByIDAndTenant(_ context.Context, _ string, _ uint64) (*types.CustomAgent, error) {
	return f.agent, f.err
}

func agentJob() *types.AgentCronJob {
	job := testJob()
	job.Mode = types.CronModeAgent
	job.AgentID = "agent-1"
	job.Prompt = "看看哪些客户要催款"
	return job
}

func emit(bus *event.EventBus, chunks ...string) {
	for _, c := range chunks {
		_ = bus.Emit(context.Background(), event.Event{
			Type: event.EventAgentFinalAnswer,
			Data: event.AgentFinalAnswerData{Content: c},
		})
	}
}

// The answer arrives in chunks; keeping only the last one would truncate every
// reply to its final fragment.
func TestAgentExec_AccumulatesStreamedAnswer(t *testing.T) {
	sessions := &fakeSessions{qa: func(_ context.Context, _ *types.QARequest, bus *event.EventBus) error {
		emit(bus, "有 3 个客户", "逾期超过 30 天", "，建议今天联系。")
		return nil
	}}
	exec := &agentExecutor{
		sessions: sessions,
		messages: &fakeMessages{},
		agents:   &fakeAgents{agent: &types.CustomAgent{ID: "agent-1"}},
		repo:     &fakeCronRepo{},
	}

	out, err := exec.Execute(context.Background(), agentJob())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "有 3 个客户逾期超过 30 天，建议今天联系。"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

// An error delivered as an event rather than a return value must not be
// recorded as an empty success — that is a run that failed while looking fine.
func TestAgentExec_CapturesErrorEvent(t *testing.T) {
	sessions := &fakeSessions{qa: func(_ context.Context, _ *types.QARequest, bus *event.EventBus) error {
		_ = bus.Emit(context.Background(), event.Event{
			Type: event.EventError,
			Data: "模型调用失败",
		})
		return nil
	}}
	exec := &agentExecutor{
		sessions: sessions,
		messages: &fakeMessages{},
		agents:   &fakeAgents{agent: &types.CustomAgent{ID: "agent-1"}},
		repo:     &fakeCronRepo{},
	}

	if _, err := exec.Execute(context.Background(), agentJob()); err == nil {
		t.Fatal("an error event was swallowed and the run looked successful")
	}
}

// One session per job, not one per run: a five-minute job would otherwise
// leave 288 sessions a day behind.
func TestAgentExec_ReusesOneSessionPerJob(t *testing.T) {
	sessions := &fakeSessions{}
	repo := &fakeCronRepo{}
	exec := &agentExecutor{
		sessions: sessions,
		messages: &fakeMessages{},
		agents:   &fakeAgents{agent: &types.CustomAgent{ID: "agent-1"}},
		repo:     repo,
	}
	job := agentJob()

	for i := 0; i < 3; i++ {
		if _, err := exec.Execute(context.Background(), job); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}

	if len(sessions.created) != 1 {
		t.Fatalf("created %d sessions across 3 runs, want 1", len(sessions.created))
	}

	// Reuse hangs entirely on the job remembering the id the session service
	// actually assigned. Asserting that the job's own id ended up on the
	// session would pass against a fake and fail against the database, which
	// is how this went unnoticed until 80 runs had left 80 sessions behind.
	assigned := sessions.created[0].ID
	if got := repo.bound[job.ID]; got != assigned {
		t.Errorf("job bound to session %q, want the assigned id %q", got, assigned)
	}
	if job.SessionID != assigned {
		t.Errorf("in-memory job.SessionID = %q, want %q — the next run would start over",
			job.SessionID, assigned)
	}
}

// Each run stores the prompt as a user turn and a placeholder assistant turn,
// so the job's history reads like a conversation rather than a wall of
// unattributed replies.
func TestAgentExec_RecordsBothTurns(t *testing.T) {
	messages := &fakeMessages{}
	exec := &agentExecutor{
		sessions: &fakeSessions{},
		messages: messages,
		agents:   &fakeAgents{agent: &types.CustomAgent{ID: "agent-1"}},
		repo:     &fakeCronRepo{},
	}
	job := agentJob()

	if _, err := exec.Execute(context.Background(), job); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(messages.created) != 2 {
		t.Fatalf("created %d messages, want 2 (user + assistant)", len(messages.created))
	}
	user, assistant := messages.created[0], messages.created[1]
	if user.Role != "user" || user.Content != job.Prompt {
		t.Errorf("user message = %+v, want the job prompt", user)
	}
	if assistant.Role != "assistant" || assistant.IsCompleted {
		t.Errorf("assistant placeholder = %+v, want an incomplete assistant turn", assistant)
	}
	if user.RequestID == "" || user.RequestID != assistant.RequestID {
		t.Error("the two turns are not tied together by a shared request id")
	}

	// Created incomplete is only half of it: a placeholder that is never
	// closed out leaves the session looking like a stream that died, and the
	// front end says so when the conversation is opened.
	if len(messages.updated) != 1 {
		t.Fatalf("updated %d messages, want the assistant turn closed out once", len(messages.updated))
	}
	if done := messages.updated[0]; done.ID != assistant.ID || !done.IsCompleted {
		t.Errorf("closed out %+v, want message %s marked complete", done, assistant.ID)
	}
}

// A job with no agent bound cannot run in agent mode, and saying so plainly
// beats a nil dereference at 3am.
func TestAgentExec_RejectsJobWithoutAgent(t *testing.T) {
	exec := &agentExecutor{
		sessions: &fakeSessions{},
		messages: &fakeMessages{},
		agents:   &fakeAgents{agent: &types.CustomAgent{ID: "agent-1"}},
		repo:     &fakeCronRepo{},
	}
	job := agentJob()
	job.AgentID = ""

	_, err := exec.Execute(context.Background(), job)
	if err == nil {
		t.Fatal("a job with no agent was accepted")
	}
	if !strings.Contains(err.Error(), "agent") {
		t.Errorf("error = %q, want it to name the missing agent", err)
	}
}

// A deleted agent must fail the run rather than silently doing nothing.
func TestAgentExec_RejectsMissingAgent(t *testing.T) {
	exec := &agentExecutor{
		sessions: &fakeSessions{},
		messages: &fakeMessages{},
		agents:   &fakeAgents{agent: nil},
		repo:     &fakeCronRepo{},
	}

	if _, err := exec.Execute(context.Background(), agentJob()); err == nil {
		t.Fatal("a job bound to a missing agent was accepted")
	}
}

// An agent with nothing to report is the watchdog case, not a failure.
func TestAgentExec_EmptyAnswerIsNotAnError(t *testing.T) {
	exec := &agentExecutor{
		sessions: &fakeSessions{},
		messages: &fakeMessages{},
		agents:   &fakeAgents{agent: &types.CustomAgent{ID: "agent-1"}},
		repo:     &fakeCronRepo{},
	}

	out, err := exec.Execute(context.Background(), agentJob())
	if err != nil {
		t.Fatalf("empty answer treated as failure: %v", err)
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

// The turn must be handed the job's prompt and the agent it is bound to,
// otherwise the run would execute against tenant defaults.
func TestAgentExec_PassesPromptAndAgent(t *testing.T) {
	sessions := &fakeSessions{}
	agent := &types.CustomAgent{ID: "agent-1", Name: "催收助手"}
	exec := &agentExecutor{
		sessions: sessions,
		messages: &fakeMessages{},
		agents:   &fakeAgents{agent: agent},
		repo:     &fakeCronRepo{},
	}
	job := agentJob()

	if _, err := exec.Execute(context.Background(), job); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	req := sessions.lastReq
	if req == nil {
		t.Fatal("AgentQA was never called")
	}
	if req.Query != job.Prompt {
		t.Errorf("query = %q, want the job prompt", req.Query)
	}
	if req.CustomAgent == nil || req.CustomAgent.ID != agent.ID {
		t.Errorf("custom agent = %+v, want %q", req.CustomAgent, agent.ID)
	}
	if req.AssistantMessageID == "" || req.UserMessageID == "" {
		t.Error("the request is missing its message ids")
	}
}
