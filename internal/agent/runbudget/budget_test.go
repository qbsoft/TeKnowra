package runbudget

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitingDoesNotConsumeActivity(t *testing.T) {
	ctx, close := New(context.Background(), Limits{Active: 80 * time.Millisecond, Wait: 300 * time.Millisecond, Wall: time.Second, MaxWaits: 1})
	defer close()
	time.Sleep(25 * time.Millisecond)
	resume, err := Pause(ctx)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if ctx.Err() != nil {
		t.Fatalf("wait consumed active time: %v", context.Cause(ctx))
	}
	resume()
	select {
	case <-ctx.Done():
		t.Fatal("activity budget reset incorrectly")
	case <-time.After(15 * time.Millisecond):
	}
	select {
	case <-ctx.Done():
		if !errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
			t.Fatal(context.Cause(ctx))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("cumulative active budget not enforced")
	}
}

func TestWaitBudgetAndSecondWait(t *testing.T) {
	ctx, close := New(context.Background(), Limits{Active: time.Second, Wait: 40 * time.Millisecond, Wall: 2 * time.Second, MaxWaits: 1})
	defer close()
	resume, err := Pause(ctx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("unbounded human wait")
	}
	resume()
	if !errors.Is(context.Cause(ctx), ErrWaitExceeded) {
		t.Fatal(context.Cause(ctx))
	}

	other, done := New(context.Background(), Limits{Active: time.Second, Wait: time.Second, Wall: 2 * time.Second, MaxWaits: 1})
	defer done()
	r, err := Pause(other)
	if err != nil {
		t.Fatal(err)
	}
	r()
	r()
	if _, err = Pause(other); !errors.Is(err, ErrWaitLimit) {
		t.Fatalf("second wait accepted: %v", err)
	}
}

func TestStoppedWaitingCannotResume(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	ctx, close := New(parent, Limits{Active: time.Second, Wait: time.Second, Wall: 2 * time.Second, MaxWaits: 1})
	defer close()
	resume, err := Pause(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	<-ctx.Done()
	resume()
	if !errors.Is(context.Cause(ctx), context.Canceled) {
		t.Fatal(context.Cause(ctx))
	}
}

func TestWallBudgetAndOrdinaryContext(t *testing.T) {
	ctx, close := New(context.Background(), Limits{Active: time.Second, Wait: time.Second, Wall: 30 * time.Millisecond, MaxWaits: 1})
	defer close()
	resume, err := Pause(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer resume()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("wall cap lost")
	}
	if !errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
		t.Fatal(context.Cause(ctx))
	}
	r, err := Pause(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r()
}

func TestApprovalSharesWaitBudgetWithoutUsingOAuthCount(t *testing.T) {
	ctx, close := New(context.Background(), Limits{Active: time.Second, Wait: time.Second, Wall: 3 * time.Second, MaxWaits: 1})
	defer close()
	r, err := PauseApproval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(15 * time.Millisecond)
	r()
	r, err = Pause(ctx)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(15 * time.Millisecond)
	r()
	stats := Snapshot(ctx)
	if stats.WaitUsed < 30*time.Millisecond || stats.ActiveUsed >= stats.WaitUsed {
		t.Fatalf("incorrect timing: %+v", stats)
	}
	if _, err = Pause(ctx); !errors.Is(err, ErrWaitLimit) {
		t.Fatal("second OAuth wait accepted")
	}
}
