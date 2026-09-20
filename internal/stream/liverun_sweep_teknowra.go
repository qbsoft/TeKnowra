package stream

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
)

// 启动时清掉上一个进程留下的「这个会话正在运行」标记 —— TeKnowra fork 自己的文件，上游没有。
//
// 问题：每轮对话开始时在 Redis 里写一个 live-run 标记，结束时在这一轮自己的 defer 里清掉。
// 进程退出时还没结束的那一轮（最典型的是停在「等待授权」上，最长 10 分钟）来不及清——优雅关闭
// 只等 ShutdownTimeout 那么久，硬杀更不用说。标记有效期 1 小时，而且每次读取都会续期
// （touchLiveRun），于是那个会话此后一直报 409 "another turn is already running in this
// session"，用户每重试一次就再续一小时，只能新建对话绕开。每晚的自动更新、白天的手动部署都会撞上。
// （2026-09-20 本机实测撞上。）
//
// 修法：我们是单实例部署。进程刚启动时不可能有任何一轮对话在跑，所以启动那一刻 Redis 里残留的
// 标记全是上一个进程留下的，直接清掉。
//
// 多实例部署不能这么做——别的实例上真有对话在跑。那时设 TEKNOWRA_SWEEP_LIVE_RUNS=false 关掉。
//
// 钩子：factory.go 的 NewStreamManager 里一行。被上游冲掉的话 liverun_sweep_teknowra_test.go 会红。

const liveRunSuffix = ":live-run"

func sweepLiveRunsEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TEKNOWRA_SWEEP_LIVE_RUNS"))) {
	case "false", "0", "off", "no":
		return false
	}
	return true
}

// sweepStaleLiveRuns 删除本前缀下所有 live-run 标记，返回删了几条。只在进程启动时调用。
func (r *RedisStreamManager) sweepStaleLiveRuns(ctx context.Context) (int, error) {
	pattern := r.prefix + ":*" + liveRunSuffix
	removed := 0
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return removed, err
		}
		for _, key := range keys {
			// SCAN 的通配只是粗筛，再核对一遍，别误删同前缀下的事件流。
			if !strings.HasSuffix(key, liveRunSuffix) {
				continue
			}
			n, err := r.client.Del(ctx, key).Result()
			if err != nil {
				return removed, err
			}
			removed += int(n)
		}
		if next == 0 {
			return removed, nil
		}
		cursor = next
	}
}

// withStartupSweep 包在 NewRedisStreamManager 外面：建好之后清一次残留标记。
// 清扫失败不影响启动——最坏情况退回到不清扫时的行为。
func withStartupSweep(mgr *RedisStreamManager, err error) (*RedisStreamManager, error) {
	if err != nil || mgr == nil || !sweepLiveRunsEnabled() {
		return mgr, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	removed, sweepErr := mgr.sweepStaleLiveRuns(ctx)
	switch {
	case sweepErr != nil:
		logger.Warnf(ctx, "[stream] startup sweep of stale live-run markers failed after %d: %v", removed, sweepErr)
	case removed > 0:
		logger.Infof(ctx, "[stream] cleared %d stale live-run marker(s) left by the previous process", removed)
	}
	return mgr, nil
}
