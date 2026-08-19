package types

import (
	"time"

	"gorm.io/gorm"
)

// Schedule kinds accepted by the cronjob tool. Whatever the user types
// ("every morning at 9", "every 30m", "0 0 9 * * *", "2h") is normalised into
// one of these plus a robfig 6-field expression before it reaches the DB.
const (
	CronScheduleOnce     = "once"
	CronScheduleInterval = "interval"
	CronScheduleCron     = "cron"
)

// Execution modes.
const (
	// CronModeAgent runs the prompt in a fresh agent session.
	CronModeAgent = "agent"
	// CronModeNoAgent runs a script/HTTP call with no LLM involved, so it
	// costs zero tokens. Empty output is a silent tick (watchdog pattern).
	CronModeNoAgent = "no_agent"
)

// Terminal statuses of the last run.
const (
	CronStatusSuccess     = "success"
	CronStatusFailed      = "failed"
	CronStatusInterrupted = "interrupted"
)

// CronPinnedModel is the provider/model snapshot taken when the job is created.
//
// Unattended jobs must not drift onto a different model because someone
// changed the global default months later: the price and the behaviour would
// both change with nobody watching. If the pin no longer matches, the run
// fails closed and alerts instead.
type CronPinnedModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// AgentCronJob is one scheduled agent task.
type AgentCronJob struct {
	ID       string `json:"id"        gorm:"type:varchar(36);primaryKey"`
	TenantID uint64 `json:"tenant_id" gorm:"index"`

	// Who asked for it. Delivery and quota are both keyed on this.
	CreatorUserID string `json:"creator_user_id" gorm:"type:varchar(64)"`

	// Which agent runs it.
	AgentID string `json:"agent_id" gorm:"type:varchar(36)"`

	// SessionID is the conversation every run appends to, so a job reads as
	// one thread instead of littering the sidebar with a session per run.
	//
	// Stored rather than derived from the job id: types.Session.BeforeCreate
	// overwrites whatever id it is given, so the session id can only be
	// learned after the fact.
	SessionID string `json:"session_id" gorm:"type:varchar(36)"`

	Name string `json:"name"`

	ScheduleKind string     `json:"schedule_kind" gorm:"type:varchar(16)"`
	ScheduleExpr string     `json:"schedule_expr" gorm:"type:varchar(128)"`
	NextRunAt    *time.Time `json:"next_run_at"`

	Prompt          string `json:"prompt"`
	Mode            string `json:"mode" gorm:"type:varchar(16)"`
	EnabledToolsets JSON   `json:"enabled_toolsets" gorm:"type:jsonb"`

	PinnedModel JSON `json:"pinned_model" gorm:"type:jsonb"`

	// nil means "run forever". Decremented only after a successful run — a
	// string of transient failures must not silently exhaust a "run it 5
	// times" budget.
	RepeatLeft *int `json:"repeat_left"`

	Enabled bool `json:"enabled"`
	Paused  bool `json:"paused"`

	LastStatus string     `json:"last_status" gorm:"type:varchar(16)"`
	LastError  string     `json:"last_error"`
	LastRunAt  *time.Time `json:"last_run_at"`

	// LastOutput is what the run produced, kept so the user has somewhere to
	// read the result of a job that fired at 3am. The scheduler does not
	// deliver anything anywhere — if a job should notify someone, that is the
	// job's own work, done through whatever tool it calls.
	LastOutput string `json:"last_output"`

	// Consecutive failures; reset on success. The nudge fires once when this
	// crosses the threshold rather than on every failed run.
	FailureStreak int `json:"failure_streak"`

	// Written by the worker at start, cleared at finish. Guards against a job
	// that runs longer than its own interval overlapping itself.
	RunningClaimBy string     `json:"running_claim_by" gorm:"type:varchar(128)"`
	RunningClaimAt *time.Time `json:"running_claim_at"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName returns the table name for the AgentCronJob model
func (j *AgentCronJob) TableName() string {
	return "agent_cron_jobs"
}

// Runnable reports whether the scheduler should consider this job at all.
func (j *AgentCronJob) Runnable() bool {
	return j.Enabled && !j.Paused
}
