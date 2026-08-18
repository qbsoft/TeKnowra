package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// AgentCronJobRepository provides data access for scheduled agent tasks.
type AgentCronJobRepository interface {
	// CreateWithQuota inserts a job, but only after checking the per-user and
	// per-tenant caps under a lock.
	//
	// The lock is not optional: the caps are a COUNT over rows that do not
	// exist yet, so two concurrent creates can both read 9, both insert, and
	// leave 11 behind. Row locks cannot serialise that — only a named lock on
	// the owner can. Returns ErrCronQuotaExceeded when a cap is hit.
	CreateWithQuota(ctx context.Context, job *types.AgentCronJob, perUser, perTenant int) error

	// FindByID retrieves a job, tenant-scoped.
	FindByID(ctx context.Context, tenantID uint64, id string) (*types.AgentCronJob, error)

	// ListByOwner lists a user's own jobs.
	ListByOwner(ctx context.Context, tenantID uint64, userID string) ([]*types.AgentCronJob, error)

	// ListRunnable loads every enabled, unpaused job across all tenants. Used
	// once at scheduler start-up to populate the cron runner.
	ListRunnable(ctx context.Context) ([]*types.AgentCronJob, error)

	// Update persists user-editable fields.
	Update(ctx context.Context, job *types.AgentCronJob) error

	// Delete soft-deletes a job.
	Delete(ctx context.Context, tenantID uint64, id string) error

	// TryClaim marks the job as running if it is not already claimed, and
	// reports whether the claim was taken. This is the overlap guard: a job
	// that runs longer than its own interval must not start a second copy.
	TryClaim(ctx context.Context, id, claimedBy string) (bool, error)

	// ReleaseClaim clears the running marker.
	ReleaseClaim(ctx context.Context, id string) error

	// SweepStaleClaims force-releases claims older than the cut-off, so a job
	// wedged by a crashed worker becomes runnable again instead of being
	// stuck forever. Returns the ids it freed so the caller can log them.
	SweepStaleClaims(ctx context.Context, olderThan time.Time) ([]string, error)

	// RecordResult writes the outcome of a run: status, error, failure streak,
	// repeat budget, and the next fire time.
	RecordResult(ctx context.Context, id string, res CronRunResult) error
}

// CronRunResult is the outcome of one execution, as persisted by the worker.
type CronRunResult struct {
	Status    string
	Error     string
	RanAt     time.Time
	NextRunAt *time.Time

	// Success decrements the repeat budget and resets the failure streak;
	// failure does neither. A run of transient errors must not burn through a
	// user's "run it 5 times".
	Success bool
}
