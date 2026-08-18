package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"
)

// staleClaimFloor is the shortest a claim may be considered stale.
//
// The real cut-off is max(2 * interval, this), so a job that legitimately runs
// for a while is not swept out from under itself, while a job wedged by a
// crashed worker still recovers on its own instead of staying dead until
// someone notices.
const staleClaimFloor = 30 * time.Minute

// sweepInterval is how often stale claims are checked. Cheap query, and the
// only cost of running it often is a wedged job recovering sooner.
const sweepInterval = "0 */5 * * * *"

// AgentCronScheduler fires user-defined scheduled agent tasks.
//
// Multi-replica de-duplication follows the pattern already established by
// internal/datasource/scheduler.go, deliberately: the platform has one working
// answer to this problem and a second one would only make the next reader
// wonder why there are two.
//
//	Layer 1 — DB:    a job still holding a running claim is skipped, so a slow
//	                 job never overlaps itself.
//	Layer 2 — Redis: robfig fires at absolute wall-clock times, so every
//	                 replica triggers in the same minute and derives the same
//	                 asynq TaskID. The first Enqueue wins; the rest get
//	                 ErrTaskIDConflict and quietly stand down.
//
// Note this is enqueue-uniqueness, not a lease: there is no TTL that can
// expire while the work is still running, which is the failure mode that
// makes TTL-based distributed locks double-fire under load.
//
// The scheduler never executes a job itself — it only enqueues. A run can take
// minutes, and holding the cron runner's goroutine for that would stall every
// other job on the instance.
type AgentCronScheduler struct {
	cron         *cron.Cron
	repo         interfaces.AgentCronJobRepository
	taskEnqueuer interfaces.TaskEnqueuer

	// instanceID identifies this replica in running claims, so a wedged job
	// says which instance dropped it.
	instanceID string

	mu      sync.Mutex
	entries map[string]cron.EntryID // jobID → cron entry
}

// NewAgentCronScheduler creates a scheduler for user-defined cron jobs.
func NewAgentCronScheduler(
	repo interfaces.AgentCronJobRepository,
	taskEnqueuer interfaces.TaskEnqueuer,
	instanceID string,
) *AgentCronScheduler {
	return &AgentCronScheduler{
		cron: cron.New(cron.WithSeconds(), cron.WithChain(
			cron.Recover(cron.DefaultLogger),
		)),
		repo:         repo,
		taskEnqueuer: taskEnqueuer,
		instanceID:   instanceID,
		entries:      make(map[string]cron.EntryID),
	}
}

// Start loads every runnable job and begins firing.
//
// Missed occurrences are NOT replayed. robfig schedules from now, so a job
// that should have fired while the process was down simply fires next time.
// The alternative — catching up — means a backend that was down for two hours
// wakes up and floods the user with 24 identical reminders.
func (s *AgentCronScheduler) Start(ctx context.Context) error {
	jobs, err := s.repo.ListRunnable(ctx)
	if err != nil {
		return fmt.Errorf("load runnable cron jobs: %w", err)
	}

	registered := 0
	for _, job := range jobs {
		if err := s.AddOrUpdate(job); err != nil {
			// One bad expression must not stop the rest from scheduling.
			logger.Errorf(ctx, "[AgentCron] skipping job=%s: %v", job.ID, err)
			continue
		}
		registered++
	}

	if _, err := s.cron.AddFunc(sweepInterval, func() { s.sweepStaleClaims() }); err != nil {
		return fmt.Errorf("register stale-claim sweeper: %w", err)
	}

	s.cron.Start()
	logger.Infof(ctx, "[AgentCron] started with %d/%d jobs registered", registered, len(jobs))
	return nil
}

// Stop halts the runner and waits for in-flight triggers to return.
func (s *AgentCronScheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

// AddOrUpdate registers or re-registers a job's schedule.
func (s *AgentCronScheduler) AddOrUpdate(job *types.AgentCronJob) error {
	if job == nil {
		return fmt.Errorf("job is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entries[job.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, job.ID)
	}
	if !job.Runnable() {
		return nil
	}

	// A one-shot job has no recurring expression to hand robfig; it is fired
	// by its next_run_at instead, expressed as a single-occurrence schedule.
	spec := job.ScheduleExpr
	if job.ScheduleKind == types.CronScheduleOnce {
		if job.NextRunAt == nil {
			return nil // already consumed
		}
		delay := time.Until(*job.NextRunAt)
		if delay <= 0 {
			delay = time.Second
		}
		spec = fmt.Sprintf("@every %s", delay)
	}

	jobID, tenantID := job.ID, job.TenantID
	entryID, err := s.cron.AddFunc(spec, func() { s.trigger(jobID, tenantID) })
	if err != nil {
		return fmt.Errorf("invalid schedule %q: %w", spec, err)
	}
	s.entries[job.ID] = entryID
	return nil
}

// Remove unregisters a job.
func (s *AgentCronScheduler) Remove(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entries[jobID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, jobID)
	}
}

// trigger runs on every tick, on every replica.
func (s *AgentCronScheduler) trigger(jobID string, tenantID uint64) {
	ctx := context.Background()

	job, err := s.repo.FindByID(ctx, tenantID, jobID)
	if err != nil {
		logger.Errorf(ctx, "[AgentCron] load job=%s failed: %v", jobID, err)
		return
	}
	if job == nil || !job.Runnable() {
		// Deleted or paused between ticks; drop the registration too.
		s.Remove(jobID)
		return
	}

	// Layer 1: a previous run is still going. Read-only on purpose — the
	// authoritative claim is taken by the worker, atomically, in TryClaim.
	if job.RunningClaimBy != "" {
		logger.Infof(ctx, "[AgentCron] job=%s still running, skipping this tick", jobID)
		return
	}

	if err := s.enqueue(ctx, job); err != nil {
		logger.Errorf(ctx, "[AgentCron] enqueue job=%s failed: %v", jobID, err)
	}
}

// EnqueueNow fires a job immediately, bypassing its schedule. Backs the
// tool's "run" action.
func (s *AgentCronScheduler) EnqueueNow(ctx context.Context, job *types.AgentCronJob) error {
	if job.RunningClaimBy != "" {
		return fmt.Errorf("这个任务正在执行中，等它跑完再试")
	}
	return s.enqueue(ctx, job)
}

func (s *AgentCronScheduler) enqueue(ctx context.Context, job *types.AgentCronJob) error {
	payload, err := json.Marshal(&types.AgentCronRunPayload{
		JobID:    job.ID,
		TenantID: job.TenantID,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// Layer 2: every replica derives the same ID within the minute, so Redis
	// admits exactly one. Minute granularity is safe because ParseSchedule
	// refuses anything faster than MinCronInterval.
	taskID := fmt.Sprintf("cronjob:%s:%s",
		job.ID, time.Now().UTC().Truncate(time.Minute).Format("200601021504"))

	_, err = s.taskEnqueuer.Enqueue(
		asynq.NewTask(types.TypeAgentCronRun, payload),
		asynq.Queue(types.QueueAgentCron),
		// One retry only. A scheduled job's real retry is its next occurrence;
		// hammering a broken job now just multiplies the cost of the failure.
		asynq.MaxRetry(1),
		asynq.Timeout(30*time.Minute),
		asynq.TaskID(taskID),
	)
	if err != nil {
		if err == asynq.ErrTaskIDConflict {
			logger.Infof(ctx, "[AgentCron] job=%s already enqueued by another replica", job.ID)
			return nil
		}
		return err
	}
	return nil
}

// sweepStaleClaims frees jobs whose worker died mid-run.
func (s *AgentCronScheduler) sweepStaleClaims() {
	ctx := context.Background()

	ids, err := s.repo.SweepStaleClaims(ctx, time.Now().Add(-staleClaimFloor))
	if err != nil {
		logger.Errorf(ctx, "[AgentCron] sweep stale claims failed: %v", err)
		return
	}
	for _, id := range ids {
		// WARNING, not Info: a wedged claim means a run vanished without
		// recording an outcome, and that is worth someone looking at.
		logger.Warnf(ctx, "[AgentCron] force-released stale claim on job=%s", id)
	}
}
