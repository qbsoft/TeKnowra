package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
)

// recordingRepo captures what the runner did, so the assertions can be about
// behaviour ("did it release the claim?") rather than about internals.
type recordingRepo struct {
	*fakeCronRepo

	mu           sync.Mutex
	claimOK      bool
	claims       int
	releases     int
	results      []interfaces.CronRunResult
	claimedBy    string
	failNextFind bool
}

func newRecordingRepo(job *types.AgentCronJob) *recordingRepo {
	return &recordingRepo{fakeCronRepo: newFakeCronRepo(job), claimOK: true}
}

func (r *recordingRepo) TryClaim(_ context.Context, _, by string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claims++
	r.claimedBy = by
	return r.claimOK, nil
}

func (r *recordingRepo) ReleaseClaim(context.Context, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.releases++
	return nil
}

func (r *recordingRepo) RecordResult(_ context.Context, _ string, res interfaces.CronRunResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, res)
	return nil
}

func (r *recordingRepo) lastResult(t *testing.T) interfaces.CronRunResult {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.results) == 0 {
		t.Fatal("no run result was recorded")
	}
	return r.results[len(r.results)-1]
}

func httpJob(t *testing.T, url, method string) *types.AgentCronJob {
	t.Helper()
	spec, err := json.Marshal(noAgentSpec{URL: url, Method: method})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	job := testJob()
	job.Mode = types.CronModeNoAgent
	job.Prompt = string(spec)
	return job
}

func runTask(t *testing.T, r *AgentCronRunner, job *types.AgentCronJob) error {
	t.Helper()
	payload, err := json.Marshal(types.AgentCronRunPayload{JobID: job.ID, TenantID: job.TenantID})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return r.Handle(context.Background(), asynq.NewTask(types.TypeAgentCronRun, payload))
}

func TestRunner_SuccessRecordsOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("  3 个客户逾期  "))
	}))
	defer srv.Close()

	job := httpJob(t, srv.URL, http.MethodGet)
	repo := newRecordingRepo(job)
	runner := NewAgentCronRunner(repo, "replica-a", 3)

	if err := runTask(t, runner, job); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	res := repo.lastResult(t)
	if !res.Success || res.Status != types.CronStatusSuccess {
		t.Errorf("status = %q success = %v, want success", res.Status, res.Success)
	}
	if res.Output != "3 个客户逾期" {
		t.Errorf("output = %q, want the trimmed body", res.Output)
	}
	if repo.releases != 1 {
		t.Errorf("claim released %d times, want 1", repo.releases)
	}
}

// A failing run must still record an outcome and still release the claim —
// otherwise the job looks like it silently stopped and stays wedged until the
// sweeper notices.
func TestRunner_FailureStillRecordsAndReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	job := httpJob(t, srv.URL, http.MethodGet)
	repo := newRecordingRepo(job)
	runner := NewAgentCronRunner(repo, "replica-a", 3)

	if err := runTask(t, runner, job); err != nil {
		t.Fatalf("Handle should swallow run failures, got: %v", err)
	}

	res := repo.lastResult(t)
	if res.Success || res.Status != types.CronStatusFailed {
		t.Errorf("status = %q success = %v, want failure", res.Status, res.Success)
	}
	if !strings.Contains(res.Error, "500") {
		t.Errorf("error = %q, want it to mention the status code", res.Error)
	}
	if repo.releases != 1 {
		t.Errorf("claim released %d times after a failure, want 1", repo.releases)
	}
}

// Losing the claim race means another worker already has it. Standing down
// must not execute anything or record a result over the winner's.
func TestRunner_StandsDownWhenClaimLost(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	job := httpJob(t, srv.URL, http.MethodGet)
	repo := newRecordingRepo(job)
	repo.claimOK = false
	runner := NewAgentCronRunner(repo, "replica-b", 3)

	if err := runTask(t, runner, job); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if hits != 0 {
		t.Errorf("executed %d times without holding the claim, want 0", hits)
	}
	if len(repo.results) != 0 {
		t.Errorf("recorded %d results without holding the claim, want 0", len(repo.results))
	}
	if repo.releases != 0 {
		t.Errorf("released a claim it never took (%d times)", repo.releases)
	}
}

// A job edited to "paused" (or deleted) while its task sat in the queue must
// not run. This is why the worker re-reads instead of trusting the payload.
func TestRunner_DropsPausedJob(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
	}))
	defer srv.Close()

	job := httpJob(t, srv.URL, http.MethodGet)
	job.Paused = true
	repo := newRecordingRepo(job)
	runner := NewAgentCronRunner(repo, "replica-a", 3)

	if err := runTask(t, runner, job); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if hits != 0 || repo.claims != 0 {
		t.Errorf("paused job ran (hits=%d claims=%d), want neither", hits, repo.claims)
	}
}

func TestRunner_RejectsBadNoAgentSpec(t *testing.T) {
	for name, prompt := range map[string]string{
		"not json":   "每天看看有没有逾期",
		"no url":     `{"method":"GET"}`,
		"empty json": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			job := testJob()
			job.Mode = types.CronModeNoAgent
			job.Prompt = prompt

			repo := newRecordingRepo(job)
			runner := NewAgentCronRunner(repo, "replica-a", 3)

			if err := runTask(t, runner, job); err != nil {
				t.Fatalf("Handle returned error: %v", err)
			}
			if res := repo.lastResult(t); res.Success {
				t.Errorf("bad spec %q was accepted", prompt)
			}
		})
	}
}

// Empty output is the watchdog pattern: "nothing to report" is the healthy
// case and must count as success, not as a broken run.
func TestRunner_EmptyOutputIsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	job := httpJob(t, srv.URL, http.MethodGet)
	repo := newRecordingRepo(job)
	runner := NewAgentCronRunner(repo, "replica-a", 3)

	if err := runTask(t, runner, job); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	res := repo.lastResult(t)
	if !res.Success {
		t.Errorf("empty output recorded as failure: %q", res.Error)
	}
	if res.Output != "" {
		t.Errorf("output = %q, want empty", res.Output)
	}
}

// A recurring job must be handed its next occurrence; a one-shot must not get
// one, which is what lets the repeat budget retire it.
func TestRunner_AdvancesScheduleOnlyForRecurring(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	t.Run("recurring", func(t *testing.T) {
		job := httpJob(t, srv.URL, http.MethodGet)
		repo := newRecordingRepo(job)
		if err := runTask(t, NewAgentCronRunner(repo, "a", 3), job); err != nil {
			t.Fatalf("Handle: %v", err)
		}
		res := repo.lastResult(t)
		if res.NextRunAt == nil || !res.NextRunAt.After(time.Now()) {
			t.Errorf("next run = %v, want a future time", res.NextRunAt)
		}
	})

	t.Run("one-shot", func(t *testing.T) {
		job := httpJob(t, srv.URL, http.MethodGet)
		job.ScheduleKind = types.CronScheduleOnce
		job.ScheduleExpr = time.Now().Add(time.Hour).Format(time.RFC3339)

		repo := newRecordingRepo(job)
		if err := runTask(t, NewAgentCronRunner(repo, "a", 3), job); err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if res := repo.lastResult(t); res.NextRunAt != nil {
			t.Errorf("one-shot got next run %v, want none", res.NextRunAt)
		}
	})
}

func TestRunner_TruncatesHugeOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", maxStoredOutput*2)))
	}))
	defer srv.Close()

	job := httpJob(t, srv.URL, http.MethodGet)
	repo := newRecordingRepo(job)
	if err := runTask(t, NewAgentCronRunner(repo, "a", 3), job); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	out := repo.lastResult(t).Output
	if len(out) <= maxStoredOutput {
		return // already bounded by the read limit
	}
	if !strings.Contains(out, "截断") {
		t.Errorf("output of %d chars was not marked as truncated", len(out))
	}
}
