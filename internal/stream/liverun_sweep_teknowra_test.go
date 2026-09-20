package stream

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newSweepTestManager(t *testing.T) (*RedisStreamManager, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	mgr, err := NewRedisStreamManager(mr.Addr(), "", "", 0, "stream:", time.Hour)
	if err != nil {
		t.Fatalf("NewRedisStreamManager: %v", err)
	}
	return mgr, mr
}

// 场景：一轮对话停在「等待授权」上时进程被重启。重启后用户再问，被上一个进程留下的标记挡住。
func TestStartupSweepUnblocksSessionLeftRunningByPreviousProcess(t *testing.T) {
	mgr, _ := newSweepTestManager(t)
	ctx := context.Background()

	if err := mgr.SetLiveRun(ctx, "session-1", "assistant-msg-1", "req-1"); err != nil {
		t.Fatalf("SetLiveRun: %v", err)
	}
	if live, _, _ := mgr.GetLiveRun(ctx, "session-1"); live != "assistant-msg-1" {
		t.Fatalf("precondition: marker should be set, got %q", live)
	}

	// 「重启」：新进程建好管理器后做的事。
	if _, err := withStartupSweep(mgr, nil); err != nil {
		t.Fatalf("withStartupSweep: %v", err)
	}

	live, _, err := mgr.GetLiveRun(ctx, "session-1")
	if err != nil || live != "" {
		t.Fatalf("会话仍被挡着：live=%q err=%v", live, err)
	}
}

func TestStartupSweepOnlyTouchesLiveRunMarkers(t *testing.T) {
	mgr, mr := newSweepTestManager(t)
	ctx := context.Background()

	for _, s := range []string{"s1", "s2", "s3"} {
		if err := mgr.SetLiveRun(ctx, s, "a-"+s, "r-"+s); err != nil {
			t.Fatalf("SetLiveRun: %v", err)
		}
	}
	// 同一个 Redis 里别的东西：事件流、别的前缀下长得像的键、名字里带 live-run 但不是标记的键。
	keep := map[string]string{
		"stream::s1:events":           "x",
		"other::s1:live-run":          "x",
		"stream::s1:live-run:history": "x",
		"teknowra:unrelated":          "x",
	}
	for k, v := range keep {
		if err := mr.Set(k, v); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := mgr.sweepStaleLiveRuns(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if removed != 3 {
		t.Errorf("removed = %d, want 3", removed)
	}
	for k := range keep {
		if !mr.Exists(k) {
			t.Errorf("误删了 %q", k)
		}
	}
}

func TestStartupSweepCanBeDisabledForMultiReplica(t *testing.T) {
	mgr, _ := newSweepTestManager(t)
	ctx := context.Background()
	if err := mgr.SetLiveRun(ctx, "session-1", "assistant-msg-1", "req-1"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEKNOWRA_SWEEP_LIVE_RUNS", "false")
	if _, err := withStartupSweep(mgr, nil); err != nil {
		t.Fatal(err)
	}
	if live, _, _ := mgr.GetLiveRun(ctx, "session-1"); live != "assistant-msg-1" {
		t.Fatal("多实例部署关掉清扫后，别的实例上正在跑的对话不该被清掉")
	}
}

func TestStartupSweepPassesConstructorErrorThrough(t *testing.T) {
	want := context.DeadlineExceeded
	if mgr, err := withStartupSweep(nil, want); err != want || mgr != nil {
		t.Fatalf("got (%v, %v)", mgr, err)
	}
}

// 钩子在上游文件 factory.go 里只有一行。被合并冲掉的话，重启后卡住的会话又回来了——不报错。
func TestFactoryWrapsRedisManagerWithStartupSweep(t *testing.T) {
	src, err := os.ReadFile("factory.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "withStartupSweep(NewRedisStreamManager(") {
		t.Fatal("factory.go no longer wraps NewRedisStreamManager with withStartupSweep")
	}
}
