//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// These exercise the parts a fake repository cannot: NULL-vs-empty-string
// semantics, the advisory lock actually serialising, and whether the claim
// predicates engage at all against the real schema.
//
//	go test -tags=integration ./internal/application/repository/ -run TestIntegrationCron
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("CRON_TEST_DSN")
	if dsn == "" {
		t.Skip("CRON_TEST_DSN not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return db
}

func newTestJob(tenantID uint64, user string) *types.AgentCronJob {
	return &types.AgentCronJob{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		CreatorUserID: user,
		Name:          "e2e",
		ScheduleKind:  types.CronScheduleCron,
		ScheduleExpr:  "0 0 9 * * *",
		Prompt:        "{}",
		Mode:          types.CronModeNoAgent,
		Enabled:       true,
	}
}

func cleanup(t *testing.T, db *gorm.DB, tenantID uint64) {
	t.Helper()
	db.Unscoped().Where("tenant_id = ?", tenantID).Delete(&types.AgentCronJob{})
}

// The claim guard is written as `running_claim_by = ”`. If the column were
// nullable, a fresh row would hold NULL, that predicate would never match, and
// the overlap guard would silently never engage — the failure mode this test
// exists to catch.
func TestIntegrationCronClaimEngages(t *testing.T) {
	db := openTestDB(t)
	repo := NewAgentCronJobRepository(db)
	ctx := context.Background()

	const tenant = 990001
	cleanup(t, db, tenant)
	defer cleanup(t, db, tenant)

	job := newTestJob(tenant, "u-claim")
	if err := repo.CreateWithQuota(ctx, job, 10, 100); err != nil {
		t.Fatalf("create: %v", err)
	}

	ok, err := repo.TryClaim(ctx, job.ID, "replica-a")
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if !ok {
		t.Fatal("first claim was refused on a fresh row — the predicate does not match reality")
	}

	ok, err = repo.TryClaim(ctx, job.ID, "replica-b")
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if ok {
		t.Fatal("second claim succeeded while the first was held — overlap guard is not engaging")
	}

	if err := repo.ReleaseClaim(ctx, job.ID); err != nil {
		t.Fatalf("release: %v", err)
	}
	ok, _ = repo.TryClaim(ctx, job.ID, "replica-c")
	if !ok {
		t.Fatal("claim refused after release")
	}
}

// Concurrent creates must not exceed the cap. Without the advisory lock both
// transactions read the same COUNT and both insert.
func TestIntegrationCronQuotaHoldsUnderConcurrency(t *testing.T) {
	db := openTestDB(t)
	repo := NewAgentCronJobRepository(db)
	ctx := context.Background()

	const (
		tenant  = 990002
		user    = "u-quota"
		limit   = 3
		writers = 12
	)
	cleanup(t, db, tenant)
	defer cleanup(t, db, tenant)

	var wg sync.WaitGroup
	start := make(chan struct{})
	accepted := make([]bool, writers)

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			err := repo.CreateWithQuota(ctx, newTestJob(tenant, user), limit, 1000)
			accepted[i] = err == nil
		}(i)
	}
	close(start)
	wg.Wait()

	var got int64
	db.Model(&types.AgentCronJob{}).Where("tenant_id = ?", tenant).Count(&got)
	if got != limit {
		t.Fatalf("%d concurrent creates against a cap of %d left %d rows", writers, limit, got)
	}
}

// A worker that dies mid-run leaves a claim behind. The sweeper has to free it
// and say so on the row, otherwise the job is stuck until someone notices.
func TestIntegrationCronSweepFreesStaleClaim(t *testing.T) {
	db := openTestDB(t)
	repo := NewAgentCronJobRepository(db)
	ctx := context.Background()

	const tenant = 990003
	cleanup(t, db, tenant)
	defer cleanup(t, db, tenant)

	job := newTestJob(tenant, "u-sweep")
	if err := repo.CreateWithQuota(ctx, job, 10, 100); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.TryClaim(ctx, job.ID, "replica-dead"); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Backdate the claim to simulate a worker that never came back.
	db.Model(&types.AgentCronJob{}).Where("id = ?", job.ID).
		Update("running_claim_at", time.Now().Add(-2*time.Hour))

	ids, err := repo.SweepStaleClaims(ctx, time.Now().Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(ids) != 1 || ids[0] != job.ID {
		t.Fatalf("sweep freed %v, want just %s", ids, job.ID)
	}

	after, _ := repo.FindByID(ctx, tenant, job.ID)
	if after.RunningClaimBy != "" {
		t.Errorf("claim still held after sweep: %q", after.RunningClaimBy)
	}
	if after.LastError == "" {
		t.Error("sweep left no breadcrumb on the row")
	}

	if ok, _ := repo.TryClaim(ctx, job.ID, "replica-new"); !ok {
		t.Error("job is still unclaimable after the sweep")
	}
}

// repeat_left must only be consumed by success, and hitting zero must disable
// rather than delete.
func TestIntegrationCronRepeatBudget(t *testing.T) {
	db := openTestDB(t)
	repo := NewAgentCronJobRepository(db)
	ctx := context.Background()

	const tenant = 990004
	cleanup(t, db, tenant)
	defer cleanup(t, db, tenant)

	job := newTestJob(tenant, "u-repeat")
	two := 2
	job.RepeatLeft = &two
	if err := repo.CreateWithQuota(ctx, job, 10, 100); err != nil {
		t.Fatalf("create: %v", err)
	}

	next := time.Now().Add(time.Hour)
	fail := interfaces.CronRunResult{
		Status: types.CronStatusFailed, Error: "boom",
		RanAt: time.Now(), NextRunAt: &next,
	}
	if err := repo.RecordResult(ctx, job.ID, fail); err != nil {
		t.Fatalf("record failure: %v", err)
	}
	got, _ := repo.FindByID(ctx, tenant, job.ID)
	if *got.RepeatLeft != 2 {
		t.Fatalf("failure consumed the budget: left=%d, want 2", *got.RepeatLeft)
	}
	if got.FailureStreak != 1 {
		t.Errorf("failure streak = %d, want 1", got.FailureStreak)
	}

	ok := interfaces.CronRunResult{
		Status: types.CronStatusSuccess, Output: "fine",
		RanAt: time.Now(), NextRunAt: &next, Success: true,
	}
	for i := 0; i < 2; i++ {
		if err := repo.RecordResult(ctx, job.ID, ok); err != nil {
			t.Fatalf("record success %d: %v", i, err)
		}
	}

	got, _ = repo.FindByID(ctx, tenant, job.ID)
	if *got.RepeatLeft != 0 {
		t.Errorf("budget left = %d after 2 successes, want 0", *got.RepeatLeft)
	}
	if got.Enabled {
		t.Error("job still enabled after its budget ran out")
	}
	if got.FailureStreak != 0 {
		t.Errorf("failure streak = %d after success, want 0", got.FailureStreak)
	}
	if got.NextRunAt != nil {
		t.Errorf("retired job still has next_run_at = %v", got.NextRunAt)
	}

	// Disabled, not deleted: the user must be able to see that it finished.
	var count int64
	db.Model(&types.AgentCronJob{}).Where("id = ?", job.ID).Count(&count)
	if count != 1 {
		t.Error("retired job disappeared instead of being disabled")
	}
}

// Ownership and tenant scoping must hold at the SQL level, not just in the
// service.
func TestIntegrationCronTenantScoping(t *testing.T) {
	db := openTestDB(t)
	repo := NewAgentCronJobRepository(db)
	ctx := context.Background()

	const tenantA, tenantB = 990005, 990006
	cleanup(t, db, tenantA)
	cleanup(t, db, tenantB)
	defer cleanup(t, db, tenantA)
	defer cleanup(t, db, tenantB)

	job := newTestJob(tenantA, "u-a")
	if err := repo.CreateWithQuota(ctx, job, 10, 100); err != nil {
		t.Fatalf("create: %v", err)
	}

	if got, _ := repo.FindByID(ctx, tenantB, job.ID); got != nil {
		t.Fatal("a job leaked across tenants")
	}
	if got, _ := repo.FindByID(ctx, tenantA, job.ID); got == nil {
		t.Fatal("job not visible to its own tenant")
	}

	if err := repo.Delete(ctx, tenantB, job.ID); err != nil {
		t.Fatalf("cross-tenant delete errored: %v", err)
	}
	if got, _ := repo.FindByID(ctx, tenantA, job.ID); got == nil {
		t.Fatal("a cross-tenant delete removed the job")
	}
}

func TestIntegrationCronListRunnableSkipsPaused(t *testing.T) {
	db := openTestDB(t)
	repo := NewAgentCronJobRepository(db)
	ctx := context.Background()

	const tenant = 990007
	cleanup(t, db, tenant)
	defer cleanup(t, db, tenant)

	live := newTestJob(tenant, "u-live")
	paused := newTestJob(tenant, "u-live")
	paused.Paused = true
	off := newTestJob(tenant, "u-live")
	off.Enabled = false

	for _, j := range []*types.AgentCronJob{live, paused, off} {
		if err := repo.CreateWithQuota(ctx, j, 10, 100); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	all, err := repo.ListRunnable(ctx)
	if err != nil {
		t.Fatalf("list runnable: %v", err)
	}
	seen := map[string]bool{}
	for _, j := range all {
		seen[j.ID] = true
	}
	if !seen[live.ID] {
		t.Error("runnable job missing from ListRunnable")
	}
	if seen[paused.ID] || seen[off.ID] {
		t.Error(fmt.Sprintf("paused/disabled jobs came back as runnable: paused=%v off=%v",
			seen[paused.ID], seen[off.ID]))
	}
}
