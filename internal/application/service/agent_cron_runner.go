package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
)

// maxStoredOutput caps what we keep from a run.
//
// The output is read back by a human through the job list, and occasionally
// fed to a model when someone asks "what did that task find yesterday". Both
// audiences are better served by a bounded excerpt than by a megabyte of log.
const maxStoredOutput = 8000

// noAgentTimeout bounds a scripted run. Long enough for a slow upstream,
// short enough that a hung endpoint does not hold the claim until the sweeper
// notices.
const noAgentTimeout = 5 * time.Minute

// cronExecutor runs a job's payload and returns what it produced.
//
// The seam exists because the two modes have nothing in common: one is an HTTP
// round-trip costing nothing, the other spins up a full agent session with a
// model behind it.
type cronExecutor interface {
	Execute(ctx context.Context, job *types.AgentCronJob) (string, error)
}

// AgentCronRunner executes scheduled jobs off the queue.
type AgentCronRunner struct {
	repo interfaces.AgentCronJobRepository

	// instanceID goes into the running claim so a wedged job names the
	// replica that dropped it.
	instanceID string

	executors map[string]cronExecutor

	// failureNudgeAt is the consecutive-failure count at which we log loudly.
	// Below it, a failing job is just a row with an error on it; nagging on
	// every single failure trains people to ignore the noise.
	failureNudgeAt int
}

// NewAgentCronRunner creates the queue-side executor for scheduled jobs.
func NewAgentCronRunner(
	repo interfaces.AgentCronJobRepository,
	instanceID string,
	failureNudgeAt int,
) *AgentCronRunner {
	if failureNudgeAt <= 0 {
		failureNudgeAt = 3
	}
	return &AgentCronRunner{
		repo:           repo,
		instanceID:     instanceID,
		failureNudgeAt: failureNudgeAt,
		executors: map[string]cronExecutor{
			types.CronModeNoAgent: &noAgentExecutor{},
		},
	}
}

// Handle is the asynq handler for types.TypeAgentCronRun.
func (r *AgentCronRunner) Handle(ctx context.Context, task *asynq.Task) error {
	var payload types.AgentCronRunPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		// Malformed payload will never become valid; returning the error would
		// only retry it. Log and drop.
		logger.Errorf(ctx, "[AgentCron] bad payload: %v", err)
		return nil
	}

	// Re-read rather than trusting the payload: the task may have sat in the
	// queue while the user edited or deleted the job, and running a prompt
	// they have since changed is worse than not running at all.
	job, err := r.repo.FindByID(ctx, payload.TenantID, payload.JobID)
	if err != nil {
		return fmt.Errorf("load job %s: %w", payload.JobID, err)
	}
	if job == nil {
		logger.Infof(ctx, "[AgentCron] job=%s is gone, dropping task", payload.JobID)
		return nil
	}
	if !job.Runnable() {
		logger.Infof(ctx, "[AgentCron] job=%s no longer runnable, dropping task", job.ID)
		return nil
	}

	// Everything downstream — message creation, session lookup, retrieval —
	// reads the caller's identity off the context. A queue worker has no
	// request to inherit it from, so the job's own owner is put back on the
	// context here, once, rather than threading it through every call.
	//
	// Missing it is not a graceful degradation: MustTenantIDFromContext
	// panics, which surfaces as a task that dies with a stack trace and no
	// row update at all.
	ctx = context.WithValue(ctx, types.TenantIDContextKey, job.TenantID)
	if job.CreatorUserID != "" {
		ctx = context.WithValue(ctx, types.UserIDContextKey, job.CreatorUserID)
	}

	// The authoritative overlap guard. The scheduler's check is a cheap
	// read; this is the atomic one, and it is what makes two workers racing
	// the same task safe.
	claimed, err := r.repo.TryClaim(ctx, job.ID, r.instanceID)
	if err != nil {
		return fmt.Errorf("claim job %s: %w", job.ID, err)
	}
	if !claimed {
		logger.Infof(ctx, "[AgentCron] job=%s already claimed elsewhere, standing down", job.ID)
		return nil
	}
	defer func() {
		if err := r.repo.ReleaseClaim(context.WithoutCancel(ctx), job.ID); err != nil {
			logger.Errorf(ctx, "[AgentCron] release claim job=%s failed: %v", job.ID, err)
		}
	}()

	output, runErr := r.execute(ctx, job)
	r.recordOutcome(ctx, job, output, runErr)

	// The outcome is recorded either way, so a failed run must not be
	// retried by asynq — its real retry is the next occurrence.
	return nil
}

func (r *AgentCronRunner) execute(ctx context.Context, job *types.AgentCronJob) (string, error) {
	mode := job.Mode
	if mode == "" {
		mode = types.CronModeAgent
	}

	exec, ok := r.executors[mode]
	if !ok {
		return "", fmt.Errorf("执行模式 %q 还不支持", mode)
	}
	return exec.Execute(ctx, job)
}

// recordOutcome persists the result and advances the schedule.
func (r *AgentCronRunner) recordOutcome(
	ctx context.Context, job *types.AgentCronJob, output string, runErr error,
) {
	// Use a context that survives the task's cancellation: a run that was cut
	// short still has an outcome worth recording, and losing it is what
	// produces jobs that look like they silently stopped.
	ctx = context.WithoutCancel(ctx)

	res := interfaces.CronRunResult{
		RanAt:   time.Now(),
		Output:  truncateOutput(output),
		Success: runErr == nil,
	}
	if runErr != nil {
		res.Status = types.CronStatusFailed
		res.Error = runErr.Error()
	} else {
		res.Status = types.CronStatusSuccess
	}

	// One-shot jobs have no next occurrence; RecordResult leaves them with a
	// nil next_run_at, and the repeat budget disables them.
	if next, err := NextAfter(job.ScheduleKind, job.ScheduleExpr, time.Now()); err == nil && !next.IsZero() {
		res.NextRunAt = &next
	}

	if err := r.repo.RecordResult(ctx, job.ID, res); err != nil {
		logger.Errorf(ctx, "[AgentCron] record result job=%s failed: %v", job.ID, err)
		return
	}

	if runErr == nil {
		return
	}

	// Nudge once, on the crossing, rather than on every failure after it.
	if streak := job.FailureStreak + 1; streak == r.failureNudgeAt {
		logger.Warnf(ctx,
			"[AgentCron] job=%s (%q) has failed %d times in a row and probably needs a look: %v",
			job.ID, job.Name, streak, runErr)
	} else {
		logger.Infof(ctx, "[AgentCron] job=%s failed: %v", job.ID, runErr)
	}
}

func truncateOutput(s string) string {
	if len(s) <= maxStoredOutput {
		return s
	}
	return s[:maxStoredOutput] + "\n…（输出过长已截断）"
}

// noAgentExecutor performs an HTTP call and returns the response body.
//
// Deliberately HTTP and not shell. hermes-agent runs cron scripts from the
// user's own machine, where "run this script" is no more privilege than the
// user already has. TeKnowra is a multi-tenant server: letting a tenant
// schedule shell commands is remote code execution offered as a feature. If
// scripted jobs are ever wanted here they belong in a sandbox, not in this
// process.
type noAgentExecutor struct{}

// noAgentSpec is the prompt payload for a no_agent job: a small JSON document
// describing the call to make.
type noAgentSpec struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

func (e *noAgentExecutor) Execute(ctx context.Context, job *types.AgentCronJob) (string, error) {
	var spec noAgentSpec
	if err := json.Unmarshal([]byte(job.Prompt), &spec); err != nil {
		return "", fmt.Errorf("no_agent 任务的内容要是一段 JSON，形如 {\"url\":\"https://...\"}：%w", err)
	}
	if spec.URL == "" {
		return "", fmt.Errorf("no_agent 任务没有给 url")
	}

	method := spec.Method
	if method == "" {
		method = http.MethodGet
	}

	ctx, cancel := context.WithTimeout(ctx, noAgentTimeout)
	defer cancel()

	var body io.Reader
	if spec.Body != "" {
		body = strings.NewReader(spec.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, spec.URL, body)
	if err != nil {
		return "", fmt.Errorf("构造请求失败：%w", err)
	}
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxStoredOutput+1))
	if err != nil {
		return "", fmt.Errorf("读取响应失败：%w", err)
	}
	out := strings.TrimSpace(string(raw))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	// Empty output is a silent tick — the watchdog pattern, where "nothing to
	// say" is the healthy case and should not manufacture a result.
	return out, nil
}
