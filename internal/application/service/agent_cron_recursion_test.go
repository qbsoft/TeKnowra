package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

func agentWithTools(names ...string) *types.CustomAgent {
	a := &types.CustomAgent{ID: "agent-1"}
	a.Config.AllowedTools = names
	return a
}

func has(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}

// A scheduled run must not be able to schedule more runs: one job whose prompt
// drifts into "and set this up to repeat" would spawn two, then four, and every
// individual job would look legitimate.
func TestWithoutSelfScheduling_StripsCronjob(t *testing.T) {
	agent := agentWithTools(tools.ToolThinking, tools.ToolCronjob, tools.ToolKnowledgeSearch)

	got := withoutSelfScheduling(agent)

	if has(got.Config.AllowedTools, tools.ToolCronjob) {
		t.Fatal("cronjob survived into a cron run — jobs can create jobs")
	}
	for _, keep := range []string{tools.ToolThinking, tools.ToolKnowledgeSearch} {
		if !has(got.Config.AllowedTools, keep) {
			t.Errorf("%s was dropped; only cronjob should be removed", keep)
		}
	}
}

// The agent comes from a service that may cache or share it. Stripping in
// place would quietly disable scheduling for the interactive users of that
// same agent.
func TestWithoutSelfScheduling_DoesNotMutateOriginal(t *testing.T) {
	agent := agentWithTools(tools.ToolThinking, tools.ToolCronjob)

	_ = withoutSelfScheduling(agent)

	if !has(agent.Config.AllowedTools, tools.ToolCronjob) {
		t.Fatal("the shared agent was mutated — interactive users just lost the tool")
	}
}

func TestWithoutSelfScheduling_NoopWhenAbsent(t *testing.T) {
	agent := agentWithTools(tools.ToolThinking)

	got := withoutSelfScheduling(agent)

	if len(got.Config.AllowedTools) != 1 {
		t.Errorf("tools = %v, want them untouched", got.Config.AllowedTools)
	}
}

// fakeSessionBinder records the session a job was bound to.
type fakeSessionBinder struct {
	*fakeCronRepo
	bound map[string]string
}

func newFakeSessionBinder(job *types.AgentCronJob) *fakeSessionBinder {
	return &fakeSessionBinder{fakeCronRepo: newFakeCronRepo(job), bound: map[string]string{}}
}

func (f *fakeSessionBinder) BindSession(_ context.Context, id, sessionID string) error {
	f.bound[id] = sessionID
	return nil
}

// A job must append to one conversation, not start a fresh one every run.
//
// The first implementation set the session id to the job id and expected to
// find it again — but types.Session.BeforeCreate overwrites any id it is
// given, so every run silently created another session. Eighty runs left
// eighty identical threads in the sidebar.
func TestEnsureSession_BindsOnceAndReuses(t *testing.T) {
	job := testJob()
	job.CreatorUserID = "u-1"

	repo := newFakeSessionBinder(job)
	sessions := &stubSessionService{created: map[string]*types.Session{}}
	exec := &agentExecutor{sessions: sessions, repo: repo}

	first, err := exec.ensureSession(context.Background(), job)
	if err != nil {
		t.Fatalf("first ensureSession: %v", err)
	}
	if repo.bound[job.ID] != first.ID {
		t.Fatalf("session %q was not bound to the job (bound=%q)", first.ID, repo.bound[job.ID])
	}

	// Second run: the job now carries the binding.
	second, err := exec.ensureSession(context.Background(), job)
	if err != nil {
		t.Fatalf("second ensureSession: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("second run used session %q, want the bound %q", second.ID, first.ID)
	}
	if sessions.createCount != 1 {
		t.Errorf("created %d sessions across two runs, want 1", sessions.createCount)
	}
}

// If the user deletes the session, the next run must still work rather than
// failing forever on a dangling reference.
func TestEnsureSession_RecoversFromDeletedSession(t *testing.T) {
	job := testJob()
	job.SessionID = "gone"

	repo := newFakeSessionBinder(job)
	sessions := &stubSessionService{created: map[string]*types.Session{}}
	exec := &agentExecutor{sessions: sessions, repo: repo}

	got, err := exec.ensureSession(context.Background(), job)
	if err != nil {
		t.Fatalf("ensureSession: %v", err)
	}
	if got.ID == "gone" {
		t.Fatal("returned the dangling session")
	}
	if repo.bound[job.ID] != got.ID {
		t.Errorf("replacement session was not re-bound")
	}
}

// stubSessionService implements only what ensureSession touches.
//
// The interface has 35 methods; embedding it nil and overriding two keeps the
// test about session reuse rather than about boilerplate. Any other method
// call would panic, which is the desired signal: ensureSession should not be
// reaching for anything else.
type stubSessionService struct {
	interfaces.SessionService
	created     map[string]*types.Session
	createCount int
	nextID      int
}

func (s *stubSessionService) CreateSession(_ context.Context, in *types.Session) (*types.Session, error) {
	s.createCount++
	s.nextID++
	// Mirror the real behaviour: the id the caller supplies is discarded.
	out := *in
	out.ID = fmt.Sprintf("session-%d", s.nextID)
	s.created[out.ID] = &out
	return &out, nil
}

func (s *stubSessionService) GetSessionByID(_ context.Context, _ uint64, id string) (*types.Session, error) {
	if sess, ok := s.created[id]; ok {
		return sess, nil
	}
	return nil, nil
}
