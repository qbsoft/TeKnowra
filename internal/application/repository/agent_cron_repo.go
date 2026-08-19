package repository

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// ErrCronQuotaExceeded is returned when a create would exceed a cap. The
// message is surfaced to the user through the agent, so it has to read like a
// sentence rather than an error code.
type ErrCronQuotaExceeded struct {
	Scope string // "user" or "tenant"
	Limit int
}

func (e *ErrCronQuotaExceeded) Error() string {
	if e.Scope == "user" {
		return fmt.Sprintf("你已经有 %d 个定时任务了，请先删掉一个再建新的", e.Limit)
	}
	return fmt.Sprintf("本工作空间的定时任务已达上限 %d 个，请联系管理员", e.Limit)
}

// AgentCronJobRepository provides data access for scheduled agent tasks.
type AgentCronJobRepository struct {
	db *gorm.DB
}

// NewAgentCronJobRepository creates a new cron job repository
func NewAgentCronJobRepository(db *gorm.DB) interfaces.AgentCronJobRepository {
	return &AgentCronJobRepository{db: db}
}

// quotaLockKey derives a stable advisory-lock key from the owner.
//
// Collisions between two unrelated users are harmless here: the worst case is
// that their two creates serialise against each other for the duration of a
// COUNT and an INSERT.
func quotaLockKey(tenantID uint64, userID string) int64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "cronquota:%d:%s", tenantID, userID)
	return int64(h.Sum64() >> 1) // keep it positive
}

// CreateWithQuota inserts a job after enforcing the per-user and per-tenant caps.
func (r *AgentCronJobRepository) CreateWithQuota(
	ctx context.Context, job *types.AgentCronJob, perUser, perTenant int,
) error {
	if job == nil {
		return errors.New("cron job is nil")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialise concurrent creates by the same owner. Without this the
		// caps below are advisory only: two requests can both see 9 and both
		// insert. Transaction-scoped so it is released on COMMIT/ROLLBACK —
		// a session-scoped lock would leak back into the connection pool.
		//
		// Postgres only. SQLite is single-writer by construction, and MySQL
		// deployments fall back to the unlocked check (the caps are a cost
		// guard, not a correctness invariant, so a rare off-by-one is
		// tolerable there).
		if tx.Dialector.Name() == "postgres" {
			key := quotaLockKey(job.TenantID, job.CreatorUserID)
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", key).Error; err != nil {
				return fmt.Errorf("take quota lock: %w", err)
			}
		}

		var userCount int64
		if err := tx.Model(&types.AgentCronJob{}).
			Where("tenant_id = ? AND creator_user_id = ?", job.TenantID, job.CreatorUserID).
			Count(&userCount).Error; err != nil {
			return err
		}
		if perUser > 0 && userCount >= int64(perUser) {
			return &ErrCronQuotaExceeded{Scope: "user", Limit: perUser}
		}

		var tenantCount int64
		if err := tx.Model(&types.AgentCronJob{}).
			Where("tenant_id = ?", job.TenantID).
			Count(&tenantCount).Error; err != nil {
			return err
		}
		if perTenant > 0 && tenantCount >= int64(perTenant) {
			return &ErrCronQuotaExceeded{Scope: "tenant", Limit: perTenant}
		}

		return tx.Create(job).Error
	})
}

// FindByID retrieves a job, tenant-scoped.
func (r *AgentCronJobRepository) FindByID(
	ctx context.Context, tenantID uint64, id string,
) (*types.AgentCronJob, error) {
	if id == "" {
		return nil, errors.New("id is empty")
	}
	var job types.AgentCronJob
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

// ListByOwner lists a user's own jobs, newest first.
func (r *AgentCronJobRepository) ListByOwner(
	ctx context.Context, tenantID uint64, userID string,
) ([]*types.AgentCronJob, error) {
	var jobs []*types.AgentCronJob
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND creator_user_id = ?", tenantID, userID).
		Order("created_at DESC").
		Find(&jobs).Error
	return jobs, err
}

// ListRunnable loads every enabled, unpaused job across all tenants.
func (r *AgentCronJobRepository) ListRunnable(ctx context.Context) ([]*types.AgentCronJob, error) {
	var jobs []*types.AgentCronJob
	err := r.db.WithContext(ctx).
		Where("enabled = ? AND paused = ?", true, false).
		Find(&jobs).Error
	return jobs, err
}

// Update persists user-editable fields.
func (r *AgentCronJobRepository) Update(ctx context.Context, job *types.AgentCronJob) error {
	if job == nil || job.ID == "" {
		return errors.New("cron job is nil or has no id")
	}
	return r.db.WithContext(ctx).
		Model(&types.AgentCronJob{}).
		Where("id = ? AND tenant_id = ?", job.ID, job.TenantID).
		Updates(map[string]interface{}{
			"name":             job.Name,
			"schedule_kind":    job.ScheduleKind,
			"schedule_expr":    job.ScheduleExpr,
			"next_run_at":      job.NextRunAt,
			"prompt":           job.Prompt,
			"mode":             job.Mode,
			"enabled_toolsets": job.EnabledToolsets,
			"repeat_left":      job.RepeatLeft,
			"enabled":          job.Enabled,
			"paused":           job.Paused,
			"updated_at":       time.Now(),
		}).Error
}

// Delete soft-deletes a job.
func (r *AgentCronJobRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&types.AgentCronJob{}).Error
}

// TryClaim marks the job as running if nobody else holds the claim.
//
// The WHERE clause carries the condition, so the check and the write are a
// single statement and two workers cannot both believe they won.
func (r *AgentCronJobRepository) TryClaim(ctx context.Context, id, claimedBy string) (bool, error) {
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&types.AgentCronJob{}).
		Where("id = ? AND running_claim_by = ''", id).
		Updates(map[string]interface{}{
			"running_claim_by": claimedBy,
			"running_claim_at": now,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ReleaseClaim clears the running marker.
func (r *AgentCronJobRepository) ReleaseClaim(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&types.AgentCronJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"running_claim_by": "",
			"running_claim_at": nil,
		}).Error
}

// SweepStaleClaims force-releases claims older than the cut-off.
func (r *AgentCronJobRepository) SweepStaleClaims(
	ctx context.Context, olderThan time.Time,
) ([]string, error) {
	var stale []*types.AgentCronJob
	err := r.db.WithContext(ctx).
		Where("running_claim_by <> '' AND running_claim_at < ?", olderThan).
		Find(&stale).Error
	if err != nil {
		return nil, err
	}
	if len(stale) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(stale))
	for _, j := range stale {
		ids = append(ids, j.ID)
	}

	// Leave a breadcrumb on the row: a wedged claim is invisible otherwise,
	// and "it just stopped running for a while" is miserable to diagnose.
	err = r.db.WithContext(ctx).
		Model(&types.AgentCronJob{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"running_claim_by": "",
			"running_claim_at": nil,
			"last_error":       "上一次执行没有正常结束，认领已被强制释放",
		}).Error
	return ids, err
}

// BindSession records which conversation a job's runs append to.
func (r *AgentCronJobRepository) BindSession(ctx context.Context, id, sessionID string) error {
	return r.db.WithContext(ctx).
		Model(&types.AgentCronJob{}).
		Where("id = ?", id).
		Update("session_id", sessionID).Error
}

// RecordResult writes the outcome of a run.
func (r *AgentCronJobRepository) RecordResult(
	ctx context.Context, id string, res interfaces.CronRunResult,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job types.AgentCronJob
		if err := tx.Where("id = ?", id).First(&job).Error; err != nil {
			return err
		}

		updates := map[string]interface{}{
			"last_status": res.Status,
			"last_error":  res.Error,
			"last_output": res.Output,
			"last_run_at": res.RanAt,
			"next_run_at": res.NextRunAt,
			"updated_at":  time.Now(),
		}

		if res.Success {
			updates["failure_streak"] = 0
			// Only a successful run consumes the budget.
			if job.RepeatLeft != nil {
				left := *job.RepeatLeft - 1
				if left < 0 {
					left = 0
				}
				updates["repeat_left"] = left
				if left == 0 {
					// Disable rather than delete: the user needs to see that
					// the job finished, not find that it vanished.
					updates["enabled"] = false
					updates["next_run_at"] = nil
				}
			}
		} else {
			updates["failure_streak"] = job.FailureStreak + 1
		}

		return tx.Model(&types.AgentCronJob{}).Where("id = ?", id).Updates(updates).Error
	})
}
