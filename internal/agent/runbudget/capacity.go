package runbudget

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
)

var ErrCapacity = errors.New("智能体执行繁忙，已达到并发上限，请稍后重试")

// Shared by both embedded engines. This is a per-process admission limit,
// not a distributed quota. Waiting for OAuth still occupies a slot.
var executionSlots struct {
	sync.Mutex
	active int
}

func Acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limit, err := strconv.Atoi(os.Getenv("TEKNOWRA_AGENT_MAX_CONCURRENT"))
	if err != nil || limit < 1 || limit > 32 {
		limit = 2
	}
	executionSlots.Lock()
	defer executionSlots.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if executionSlots.active >= limit {
		return nil, ErrCapacity
	}
	executionSlots.active++
	var once sync.Once
	return func() { once.Do(func() { executionSlots.Lock(); executionSlots.active--; executionSlots.Unlock() }) }, nil
}
