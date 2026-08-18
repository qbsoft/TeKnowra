package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad fixture time %q: %v", s, err)
	}
	return v
}

func TestParseSchedule_Accepts(t *testing.T) {
	now := mustTime(t, "2026-08-16T10:00:00Z")

	cases := []struct {
		name     string
		in       string
		wantKind string
		wantNext string
	}{
		{"6-field cron", "0 0 9 * * *", types.CronScheduleCron, "2026-08-17T09:00:00Z"},
		{"5-field cron gets seconds", "0 9 * * *", types.CronScheduleCron, "2026-08-17T09:00:00Z"},
		{"descriptor", "@daily", types.CronScheduleCron, "2026-08-17T00:00:00Z"},
		{"interval", "every 30m", types.CronScheduleInterval, "2026-08-16T10:30:00Z"},
		{"one-shot relative", "2h", types.CronScheduleOnce, "2026-08-16T12:00:00Z"},
		{"one-shot absolute", "2026-09-01T08:30:00Z", types.CronScheduleOnce, "2026-09-01T08:30:00Z"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseSchedule(c.in, now)
			if err != nil {
				t.Fatalf("ParseSchedule(%q) failed: %v", c.in, err)
			}
			if got.Kind != c.wantKind {
				t.Errorf("kind = %q, want %q", got.Kind, c.wantKind)
			}
			if want := mustTime(t, c.wantNext); !got.Next.Equal(want) {
				t.Errorf("next = %v, want %v", got.Next, want)
			}
		})
	}
}

// The floor is a cost guard, so it has to hold for every way of expressing a
// too-frequent schedule, not just the obvious one.
func TestParseSchedule_RejectsTooFrequent(t *testing.T) {
	now := mustTime(t, "2026-08-16T10:00:00Z")

	for _, in := range []string{
		"every 1s",
		"every 30s",
		"every 1m",
		"* * * * * *",   // every second
		"0 * * * * *",   // every minute
		"0 */2 * * * *", // every two minutes
	} {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseSchedule(in, now); err == nil {
				t.Fatalf("ParseSchedule(%q) should have been rejected", in)
			}
		})
	}
}

func TestParseSchedule_RejectsGarbageAndPast(t *testing.T) {
	now := mustTime(t, "2026-08-16T10:00:00Z")

	for _, in := range []string{
		"",
		"每天早上九点", // natural language is the agent's job, not ours
		"soon",
		"0 0 9 *",              // 4 fields: too few to mean anything
		"0 0 9 * * * *",        // 7 fields: too many
		"2020-01-01T00:00:00Z", // in the past
		"-2h",                  // negative
	} {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseSchedule(in, now); err == nil {
				t.Fatalf("ParseSchedule(%q) should have been rejected", in)
			}
		})
	}
}

// A one-shot job has no next occurrence; the caller relies on that to disable
// it after it runs instead of rescheduling it forever.
func TestNextAfter_OnceHasNoNext(t *testing.T) {
	now := mustTime(t, "2026-08-16T10:00:00Z")

	next, err := NextAfter(types.CronScheduleOnce, now.Format(time.RFC3339), now)
	if err != nil {
		t.Fatalf("NextAfter failed: %v", err)
	}
	if !next.IsZero() {
		t.Errorf("one-shot next = %v, want zero", next)
	}
}

func TestNextAfter_Recurring(t *testing.T) {
	now := mustTime(t, "2026-08-16T10:00:00Z")

	next, err := NextAfter(types.CronScheduleCron, "0 0 9 * * *", now)
	if err != nil {
		t.Fatalf("NextAfter failed: %v", err)
	}
	if want := mustTime(t, "2026-08-17T09:00:00Z"); !next.Equal(want) {
		t.Errorf("next = %v, want %v", next, want)
	}
}

// Intervals must land on absolute wall-clock times wherever that is possible.
//
// robfig schedules "@every" from the moment of registration, so two replicas
// that started minutes apart fire at different minutes, derive different dedup
// keys, and both run — defeating the scheduler's de-duplication. Converting to
// a cron expression removes the whole class of problem.
func TestParseSchedule_IntervalsBecomeAbsolute(t *testing.T) {
	now := mustTime(t, "2026-08-16T10:03:00Z")

	cases := []struct {
		in       string
		wantExpr string
		wantNext string
	}{
		{"every 5m", "0 */5 * * * *", "2026-08-16T10:05:00Z"},
		{"every 10m", "0 */10 * * * *", "2026-08-16T10:10:00Z"},
		{"every 30m", "0 */30 * * * *", "2026-08-16T10:30:00Z"},
		{"every 1h", "0 0 */1 * * *", "2026-08-16T11:00:00Z"},
		{"every 6h", "0 0 */6 * * *", "2026-08-16T12:00:00Z"},
		{"every 24h", "0 0 0 * * *", "2026-08-17T00:00:00Z"},
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseSchedule(c.in, now)
			if err != nil {
				t.Fatalf("ParseSchedule(%q): %v", c.in, err)
			}
			if got.Expr != c.wantExpr {
				t.Errorf("expr = %q, want %q", got.Expr, c.wantExpr)
			}
			if want := mustTime(t, c.wantNext); !got.Next.Equal(want) {
				t.Errorf("next = %v, want %v (aligned to the clock, not to now)", got.Next, want)
			}
		})
	}
}

// An interval that does not divide the hour cannot be expressed on the clock.
// Rewriting it anyway would change when the job runs, so it keeps the relative
// form and gives up absolute alignment.
func TestParseSchedule_IndivisibleIntervalStaysRelative(t *testing.T) {
	now := mustTime(t, "2026-08-16T10:03:00Z")

	for _, in := range []string{"every 7m", "every 90s", "every 5h"} {
		t.Run(in, func(t *testing.T) {
			got, err := ParseSchedule(in, now)
			if err != nil {
				t.Skipf("%q is rejected for another reason: %v", in, err)
			}
			if !strings.HasPrefix(got.Expr, "@every") {
				t.Errorf("expr = %q, want the relative form", got.Expr)
			}
		})
	}
}
