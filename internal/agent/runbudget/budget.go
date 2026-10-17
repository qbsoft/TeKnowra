// Package runbudget separates human waiting from cumulative active execution.
package runbudget

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrWaitExceeded = errors.New("授权等待总预算已耗尽")
	ErrWaitLimit    = errors.New("本次任务已使用重新授权次数，请重新发起")
)

type Limits struct {
	Active, Wait, Wall time.Duration
	MaxWaits           int
}

var Default = Limits{Active: 120 * time.Second, Wait: 600 * time.Second, Wall: 720 * time.Second, MaxWaits: 1}

type budgetKey struct{}
type budget struct {
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelCauseFunc
	active, wait    time.Duration
	limits          Limits
	created         time.Time
	started         time.Time
	waits, maxWaits int
	paused          bool
	generation      uint64
	timer           *time.Timer
}

func New(parent context.Context, limits Limits) (context.Context, context.CancelFunc) {
	if limits.Active <= 0 || limits.Wait <= 0 || limits.Wall <= 0 || limits.MaxWaits <= 0 {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, cancel
	}
	wall, wallCancel := context.WithTimeout(parent, limits.Wall)
	ctx, cancel := context.WithCancelCause(wall)
	b := &budget{ctx: ctx, cancel: cancel, active: limits.Active, wait: limits.Wait, maxWaits: limits.MaxWaits, started: time.Now(), created: time.Now(), limits: limits}
	b.arm(limits.Active, context.DeadlineExceeded)
	cleanup := func() {
		b.mu.Lock()
		b.generation++
		b.timer.Stop()
		b.mu.Unlock()
		cancel(context.Canceled)
		wallCancel()
	}
	context.AfterFunc(ctx, cleanup)
	return context.WithValue(ctx, budgetKey{}, b), cleanup
}

// arm is called while constructing the budget or holding its lock. Generation
// checks prevent a racing, stopped activity timer from canceling a human wait.
func (b *budget) arm(duration time.Duration, cause error) {
	b.generation++
	gen := b.generation
	b.timer = time.AfterFunc(duration, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if gen == b.generation {
			b.cancel(cause)
		}
	})
}

// Pause is a no-op for ordinary agents. The returned resume is idempotent and
// cannot revive a canceled run or reset its remaining cumulative active time.
func Pause(ctx context.Context) (func(), error) {
	return pause(ctx, true)
}

// Approval waiting shares the human wait quota, but is not OAuth reauthorization.
func PauseApproval(ctx context.Context) (func(), error) { return pause(ctx, false) }

func pause(ctx context.Context, countOAuth bool) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, context.Cause(ctx)
	}
	b, ok := ctx.Value(budgetKey{}).(*budget)
	if !ok {
		return func() {}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.ctx.Err(); err != nil {
		return nil, context.Cause(b.ctx)
	}
	if b.paused || (countOAuth && b.waits >= b.maxWaits) {
		return nil, ErrWaitLimit
	}
	b.active -= time.Since(b.started)
	if b.active <= 0 {
		b.cancel(context.DeadlineExceeded)
		return nil, context.DeadlineExceeded
	}
	b.timer.Stop()
	if countOAuth {
		b.waits++
	}
	b.paused = true
	b.started = time.Now()
	b.arm(b.wait, ErrWaitExceeded)
	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			b.timer.Stop()
			b.generation++
			b.wait -= time.Since(b.started)
			b.paused = false
			if b.ctx.Err() != nil {
				return
			}
			if b.wait <= 0 {
				b.cancel(ErrWaitExceeded)
				return
			}
			b.started = time.Now()
			b.arm(b.active, context.DeadlineExceeded)
		})
	}, nil
}

type Usage struct{ ActiveUsed, WaitUsed, Elapsed time.Duration }

func Snapshot(ctx context.Context) Usage {
	b, ok := ctx.Value(budgetKey{}).(*budget)
	if !ok {
		return Usage{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	active, wait := b.limits.Active-b.active, b.limits.Wait-b.wait
	if b.paused {
		wait += time.Since(b.started)
	} else {
		active += time.Since(b.started)
	}
	return Usage{ActiveUsed: min(active, b.limits.Active), WaitUsed: min(wait, b.limits.Wait), Elapsed: time.Since(b.created)}
}
