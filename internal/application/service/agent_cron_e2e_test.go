//go:build integration

package service

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestE2ECronFiresThroughLiveWorker drives the whole path with nothing faked:
// a real row in Postgres, a real task on the real Redis queue, executed by the
// backend process that is actually running, writing its result back to the row.
//
// The unit tests all use a fake repository and a fake enqueuer, so none of them
// can tell whether the pieces agree with each other. This one can.
//
//	CRON_TEST_DSN=... CRON_TEST_REDIS=... CRON_TEST_TARGET=http://127.0.0.1:8080/health \
//	  go test -tags=integration ./internal/application/service/ -run TestE2ECron -v
func TestE2ECronFiresThroughLiveWorker(t *testing.T) {
	dsn := os.Getenv("CRON_TEST_DSN")
	redisAddr := os.Getenv("CRON_TEST_REDIS")
	target := os.Getenv("CRON_TEST_TARGET")
	if dsn == "" || redisAddr == "" || target == "" {
		t.Skip("CRON_TEST_DSN / CRON_TEST_REDIS / CRON_TEST_TARGET not all set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	repo := repository.NewAgentCronJobRepository(db)
	ctx := context.Background()

	const tenant = 990100
	db.Unscoped().Where("tenant_id = ?", tenant).Delete(&types.AgentCronJob{})
	defer db.Unscoped().Where("tenant_id = ?", tenant).Delete(&types.AgentCronJob{})

	spec, _ := json.Marshal(map[string]string{"url": target})
	job := &types.AgentCronJob{
		ID:            uuid.New().String(),
		TenantID:      tenant,
		CreatorUserID: "u-e2e",
		Name:          "e2e 探活",
		ScheduleKind:  types.CronScheduleCron,
		ScheduleExpr:  "0 0 9 * * *",
		Prompt:        string(spec),
		Mode:          types.CronModeNoAgent,
		Enabled:       true,
	}
	if err := repo.CreateWithQuota(ctx, job, 10, 100); err != nil {
		t.Fatalf("create job: %v", err)
	}

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: os.Getenv("CRON_TEST_REDIS_PASSWORD"),
	})
	defer client.Close()

	payload, _ := json.Marshal(types.AgentCronRunPayload{JobID: job.ID, TenantID: tenant})
	taskID := "cronjob:" + job.ID + ":" + time.Now().UTC().Format("200601021504")

	if _, err := client.Enqueue(
		asynq.NewTask(types.TypeAgentCronRun, payload),
		asynq.Queue(types.QueueAgentCron),
		asynq.TaskID(taskID),
		asynq.MaxRetry(1),
		asynq.Timeout(2*time.Minute),
	); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// The live worker picks it up on its own schedule; poll rather than guess.
	deadline := time.Now().Add(90 * time.Second)
	var got *types.AgentCronJob
	for time.Now().Before(deadline) {
		got, err = repo.FindByID(ctx, tenant, job.ID)
		if err != nil {
			t.Fatalf("reload job: %v", err)
		}
		if got != nil && got.LastRunAt != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if got == nil || got.LastRunAt == nil {
		t.Fatal("the live worker never ran the job — check that the backend is up " +
			"with WEKNORA_AGENT_CRON_ENABLED=true and that it consumes the agent_cron queue")
	}
	if got.LastStatus != types.CronStatusSuccess {
		t.Fatalf("run failed: status=%s err=%s", got.LastStatus, got.LastError)
	}
	if !strings.Contains(got.LastOutput, "ok") {
		t.Errorf("output = %q, want the health payload", got.LastOutput)
	}

	// The claim must be handed back, or the job would never run again.
	if got.RunningClaimBy != "" {
		t.Errorf("claim still held after the run: %q", got.RunningClaimBy)
	}
	// A recurring job must have been given its next occurrence.
	if got.NextRunAt == nil || !got.NextRunAt.After(time.Now()) {
		t.Errorf("next run = %v, want a future time", got.NextRunAt)
	}
	if got.FailureStreak != 0 {
		t.Errorf("failure streak = %d after a success, want 0", got.FailureStreak)
	}

	t.Logf("live worker ran the job: status=%s output=%q next=%v",
		got.LastStatus, got.LastOutput, got.NextRunAt.Local().Format("2006-01-02 15:04"))
}
