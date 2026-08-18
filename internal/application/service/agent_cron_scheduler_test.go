package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
)

// fakeEnqueuer stands in for asynq + Redis, enforcing the one property the
// de-duplication actually rests on: a TaskID may be enqueued once.
type fakeEnqueuer struct {
	mu   sync.Mutex
	seen map[string]int // taskID → accepted count
	ids  []string
}

func newFakeEnqueuer() *fakeEnqueuer {
	return &fakeEnqueuer{seen: make(map[string]int)}
}

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	var taskID string
	for _, o := range opts {
		if o.Type() == asynq.TaskIDOpt {
			if v, ok := o.Value().(string); ok {
				taskID = v
			}
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if taskID != "" {
		if _, exists := f.seen[taskID]; exists {
			return nil, asynq.ErrTaskIDConflict
		}
		f.seen[taskID] = 1
		f.ids = append(f.ids, taskID)
	}
	return &asynq.TaskInfo{ID: taskID}, nil
}

func (f *fakeEnqueuer) accepted() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.ids)
}

// fakeCronRepo is a minimal in-memory stand-in; only the methods the
// scheduler touches are meaningful.
type fakeCronRepo struct {
	mu   sync.Mutex
	jobs map[string]*types.AgentCronJob
}

func newFakeCronRepo(jobs ...*types.AgentCronJob) *fakeCronRepo {
	r := &fakeCronRepo{jobs: make(map[string]*types.AgentCronJob)}
	for _, j := range jobs {
		r.jobs[j.ID] = j
	}
	return r
}

func (r *fakeCronRepo) FindByID(_ context.Context, _ uint64, id string) (*types.AgentCronJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.jobs[id], nil
}

func (r *fakeCronRepo) CreateWithQuota(context.Context, *types.AgentCronJob, int, int) error {
	return nil
}
func (r *fakeCronRepo) ListByOwner(context.Context, uint64, string) ([]*types.AgentCronJob, error) {
	return nil, nil
}
func (r *fakeCronRepo) ListRunnable(context.Context) ([]*types.AgentCronJob, error) {
	return nil, nil
}
func (r *fakeCronRepo) Update(context.Context, *types.AgentCronJob) error      { return nil }
func (r *fakeCronRepo) Delete(context.Context, uint64, string) error           { return nil }
func (r *fakeCronRepo) TryClaim(context.Context, string, string) (bool, error) { return true, nil }
func (r *fakeCronRepo) ReleaseClaim(context.Context, string) error             { return nil }
func (r *fakeCronRepo) SweepStaleClaims(context.Context, time.Time) ([]string, error) {
	return nil, nil
}
func (r *fakeCronRepo) RecordResult(context.Context, string, interfaces.CronRunResult) error {
	return nil
}

func testJob() *types.AgentCronJob {
	return &types.AgentCronJob{
		ID:            "job-1",
		TenantID:      7,
		CreatorUserID: "u-1",
		ScheduleKind:  types.CronScheduleCron,
		ScheduleExpr:  "0 0 9 * * *",
		Enabled:       true,
	}
}

// The whole multi-replica story rests on this: N replicas firing the same job
// in the same minute must produce exactly one unit of work.
func TestScheduler_ConcurrentReplicasEnqueueOnce(t *testing.T) {
	const replicas = 8

	job := testJob()
	repo := newFakeCronRepo(job)
	enq := newFakeEnqueuer() // shared, like one Redis behind many replicas

	schedulers := make([]*AgentCronScheduler, replicas)
	for i := range schedulers {
		schedulers[i] = NewAgentCronScheduler(repo, enq, "replica-"+string(rune('a'+i)))
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, s := range schedulers {
		wg.Add(1)
		go func(s *AgentCronScheduler) {
			defer wg.Done()
			<-start // fire as simultaneously as the runtime allows
			s.trigger(job.ID, job.TenantID)
		}(s)
	}
	close(start)
	wg.Wait()

	if got := enq.accepted(); got != 1 {
		t.Fatalf("%d replicas produced %d enqueues, want exactly 1", replicas, got)
	}
}

// Layer 1: a job whose previous run has not finished must not be started again,
// no matter how many ticks go by.
func TestScheduler_SkipsWhileRunning(t *testing.T) {
	job := testJob()
	claimedAt := time.Now()
	job.RunningClaimBy = "replica-a"
	job.RunningClaimAt = &claimedAt

	repo := newFakeCronRepo(job)
	enq := newFakeEnqueuer()
	s := NewAgentCronScheduler(repo, enq, "replica-b")

	s.trigger(job.ID, job.TenantID)
	s.trigger(job.ID, job.TenantID)

	if got := enq.accepted(); got != 0 {
		t.Fatalf("enqueued %d times while a run was in flight, want 0", got)
	}
}

// A paused or deleted job must stop firing, and stop being registered.
func TestScheduler_DropsUnrunnableJob(t *testing.T) {
	job := testJob()
	job.Paused = true

	repo := newFakeCronRepo(job)
	enq := newFakeEnqueuer()
	s := NewAgentCronScheduler(repo, enq, "replica-a")

	s.trigger(job.ID, job.TenantID)

	if got := enq.accepted(); got != 0 {
		t.Fatalf("paused job enqueued %d times, want 0", got)
	}
}

// Registering the same job twice must replace the entry rather than double it,
// otherwise an edit would leave the old schedule firing alongside the new one.
func TestScheduler_AddOrUpdateReplacesEntry(t *testing.T) {
	job := testJob()
	s := NewAgentCronScheduler(newFakeCronRepo(job), newFakeEnqueuer(), "replica-a")

	for i := 0; i < 3; i++ {
		if err := s.AddOrUpdate(job); err != nil {
			t.Fatalf("AddOrUpdate failed: %v", err)
		}
	}
	if got := len(s.cron.Entries()); got != 1 {
		t.Fatalf("cron has %d entries after 3 registrations, want 1", got)
	}

	s.Remove(job.ID)
	if got := len(s.cron.Entries()); got != 0 {
		t.Fatalf("cron has %d entries after Remove, want 0", got)
	}
}
