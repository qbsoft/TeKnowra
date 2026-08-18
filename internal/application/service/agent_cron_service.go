package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

// Default quotas. Deliberately low: they exist so one person who has not
// thought it through cannot spend a tenant's budget overnight, and raising a
// limit is a much easier conversation than explaining a bill.
const (
	defaultJobsPerUser   = 10
	defaultJobsPerTenant = 200
)

// AgentCronService is what the cronjob tool talks to.
type AgentCronService struct {
	repo      interfaces.AgentCronJobRepository
	scheduler *AgentCronScheduler

	jobsPerUser   int
	jobsPerTenant int
}

// Compile-time proof the service satisfies what the tool depends on.
var _ interfaces.AgentCronManager = (*AgentCronService)(nil)

// NewAgentCronService creates the service backing the cronjob tool.
func NewAgentCronService(
	repo interfaces.AgentCronJobRepository,
	scheduler *AgentCronScheduler,
	jobsPerUser, jobsPerTenant int,
) *AgentCronService {
	if jobsPerUser <= 0 {
		jobsPerUser = defaultJobsPerUser
	}
	if jobsPerTenant <= 0 {
		jobsPerTenant = defaultJobsPerTenant
	}
	return &AgentCronService{
		repo:          repo,
		scheduler:     scheduler,
		jobsPerUser:   jobsPerUser,
		jobsPerTenant: jobsPerTenant,
	}
}

// Create validates, persists and schedules a new job.
func (s *AgentCronService) Create(ctx context.Context, in interfaces.CreateJobInput) (*types.AgentCronJob, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("找不到当前工作空间，无法创建定时任务")
	}
	userID, _ := types.UserIDFromContext(ctx)
	if userID == "" {
		// Without an owner there is nobody to bill the quota to and nobody to
		// show the job to. Better to refuse than to create an orphan that
		// nobody can find or stop.
		return nil, fmt.Errorf("找不到当前用户身份，无法创建定时任务")
	}

	if strings.TrimSpace(in.Prompt) == "" {
		return nil, fmt.Errorf("定时任务得说清楚要做什么")
	}

	sched, err := ParseSchedule(in.Schedule, time.Now())
	if err != nil {
		return nil, err
	}

	mode := in.Mode
	if mode == "" {
		mode = types.CronModeAgent
	}
	if mode != types.CronModeAgent && mode != types.CronModeNoAgent {
		return nil, fmt.Errorf("执行模式只能是 %q 或 %q", types.CronModeAgent, types.CronModeNoAgent)
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = summarise(in.Prompt)
	}

	job := &types.AgentCronJob{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		CreatorUserID: userID,
		AgentID:       in.AgentID,
		Name:          name,
		ScheduleKind:  sched.Kind,
		ScheduleExpr:  sched.Expr,
		NextRunAt:     &sched.Next,
		Prompt:        in.Prompt,
		Mode:          mode,
		Enabled:       true,
	}

	// A one-shot job is just a repeat budget of 1; keeping them the same
	// mechanism means "runs out and retires" has one implementation.
	switch {
	case in.Repeat > 0:
		r := in.Repeat
		job.RepeatLeft = &r
	case sched.Kind == types.CronScheduleOnce:
		one := 1
		job.RepeatLeft = &one
	}

	if err := s.repo.CreateWithQuota(ctx, job, s.jobsPerUser, s.jobsPerTenant); err != nil {
		return nil, err
	}
	if err := s.scheduler.AddOrUpdate(job); err != nil {
		// The row exists but nothing will fire it; deleting is kinder than
		// leaving a job the user believes is scheduled.
		_ = s.repo.Delete(ctx, tenantID, job.ID)
		return nil, fmt.Errorf("排程失败：%w", err)
	}
	return job, nil
}

// List returns the caller's own jobs.
func (s *AgentCronService) List(ctx context.Context) ([]*types.AgentCronJob, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("找不到当前工作空间")
	}
	userID, _ := types.UserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("找不到当前用户身份")
	}
	return s.repo.ListByOwner(ctx, tenantID, userID)
}

// get loads a job the caller is allowed to touch.
//
// Ownership is checked here rather than in the repository so every action goes
// through the same gate: a user may only ever act on their own jobs, and the
// model cannot talk its way past it by passing someone else's id.
func (s *AgentCronService) get(ctx context.Context, id string) (*types.AgentCronJob, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("找不到当前工作空间")
	}
	userID, _ := types.UserIDFromContext(ctx)

	job, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if job == nil || job.CreatorUserID != userID {
		// Same message either way: "exists but is not yours" would leak that
		// the id is real.
		return nil, fmt.Errorf("没有找到这个定时任务")
	}
	return job, nil
}

// SetPaused pauses or resumes a job.
func (s *AgentCronService) SetPaused(ctx context.Context, id string, paused bool) (*types.AgentCronJob, error) {
	job, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	job.Paused = paused
	if err := s.repo.Update(ctx, job); err != nil {
		return nil, err
	}
	if err := s.scheduler.AddOrUpdate(job); err != nil {
		return nil, err
	}
	return job, nil
}

// Remove deletes a job.
func (s *AgentCronService) Remove(ctx context.Context, id string) (*types.AgentCronJob, error) {
	job, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, job.TenantID, job.ID); err != nil {
		return nil, err
	}
	s.scheduler.Remove(job.ID)
	return job, nil
}

// RunNow fires a job immediately without touching its schedule.
func (s *AgentCronService) RunNow(ctx context.Context, id string) (*types.AgentCronJob, error) {
	job, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.scheduler.EnqueueNow(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// Update changes an existing job.
func (s *AgentCronService) Update(ctx context.Context, id string, in interfaces.UpdateJobInput) (*types.AgentCronJob, error) {
	job, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}

	// A consumed one-shot has already happened; editing it would silently
	// resurrect something the user considers finished.
	if job.ScheduleKind == types.CronScheduleOnce && job.NextRunAt == nil {
		return nil, fmt.Errorf("这个一次性任务已经执行过了，改不了，可以新建一个")
	}

	if in.Schedule != "" {
		sched, err := ParseSchedule(in.Schedule, time.Now())
		if err != nil {
			return nil, err
		}
		job.ScheduleKind = sched.Kind
		job.ScheduleExpr = sched.Expr
		job.NextRunAt = &sched.Next
	}
	if in.Prompt != "" {
		job.Prompt = in.Prompt
	}
	if in.Name != "" {
		job.Name = strings.TrimSpace(in.Name)
	}

	if err := s.repo.Update(ctx, job); err != nil {
		return nil, err
	}
	if err := s.scheduler.AddOrUpdate(job); err != nil {
		return nil, err
	}
	return job, nil
}

// IsQuotaError reports whether the error is a quota rejection, which the tool
// surfaces verbatim because the message is written for the end user.
func IsQuotaError(err error) bool {
	var q *repository.ErrCronQuotaExceeded
	return errors.As(err, &q)
}

// summarise derives a short name from the prompt so a job is recognisable in a
// list without the user having to name it.
func summarise(prompt string) string {
	s := strings.TrimSpace(strings.ReplaceAll(prompt, "\n", " "))
	runes := []rune(s)
	if len(runes) > 20 {
		return string(runes[:20]) + "…"
	}
	return s
}
