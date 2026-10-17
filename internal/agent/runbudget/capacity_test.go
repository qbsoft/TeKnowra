package runbudget

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDualEngineCapacitySharedAndReleased(t *testing.T) {
	t.Setenv("TEKNOWRA_AGENT_MAX_CONCURRENT", "2")
	eino, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer eino()
	langflow, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer langflow()
	if _, err := Acquire(context.Background()); !errors.Is(err, ErrCapacity) {
		t.Fatalf("third task should reject: %v", err)
	}
	eino()
	eino()
	replacement, err := Acquire(context.Background())
	if err != nil {
		t.Fatal("released slot leaked", err)
	}
	replacement()
}

func TestDualEngineCapacityCanceledDoesNotConsumeSlot(t *testing.T) {
	t.Setenv("TEKNOWRA_AGENT_MAX_CONCURRENT", "1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	release, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestDualEngineCapacityTimeoutReleasesViaDefer(t *testing.T) {
	t.Setenv("TEKNOWRA_AGENT_MAX_CONCURRENT", "1")
	err := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		release, err := Acquire(ctx)
		if err != nil {
			return err
		}
		defer release()
		<-ctx.Done()
		return ctx.Err()
	}()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	release, err := Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
}
