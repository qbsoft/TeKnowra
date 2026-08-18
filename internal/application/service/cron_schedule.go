package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/robfig/cron/v3"
)

// cronParser matches the runner's dialect: 6 fields, seconds first.
var cronParser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// MinCronInterval is the floor between two runs of the same job.
//
// It exists for cost, not correctness: an agent run costs tokens, and "every
// second" from someone who did not think it through would quietly burn a
// tenant's budget overnight. It also makes minute-granularity dedup keys safe
// (see the scheduler), which is a happy accident rather than the reason.
const MinCronInterval = 5 * time.Minute

// ParsedSchedule is the normalised form of whatever the user asked for.
type ParsedSchedule struct {
	Kind string // types.CronScheduleOnce | Interval | Cron
	Expr string // robfig 6-field expression, or an RFC3339 instant for "once"
	Next time.Time
}

// ParseSchedule normalises a schedule string.
//
// Deliberately NOT a natural-language parser. The agent is told to turn "每天
// 早上9点" into "0 0 9 * * *" itself — it is far better at that than any
// pattern table we would maintain, and a wrong guess here is invisible until
// the job fires at the wrong hour. What this function does is validate, reject
// anything ambiguous, and enforce the floor.
//
// Accepted:
//
//	"0 0 9 * * *"    6-field cron (seconds first)
//	"0 9 * * *"      5-field cron, seconds assumed 0
//	"@daily"         robfig descriptors
//	"every 30m"      recurring interval
//	"30m"            one-shot, relative to now
//	RFC3339          one-shot, absolute
func ParseSchedule(raw string, now time.Time) (*ParsedSchedule, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return nil, fmt.Errorf("没有给出执行时间")
	}

	// Recurring interval: "every 30m"
	if rest, ok := strings.CutPrefix(s, "every "); ok {
		d, err := time.ParseDuration(strings.TrimSpace(rest))
		if err != nil {
			return nil, fmt.Errorf("看不懂的间隔 %q，试试 \"every 30m\" 或 cron 表达式", raw)
		}
		if d < MinCronInterval {
			return nil, fmt.Errorf("间隔太短了，最快只能每 %v 一次", MinCronInterval)
		}
		return &ParsedSchedule{
			Kind: types.CronScheduleInterval,
			Expr: fmt.Sprintf("@every %s", d),
			Next: now.Add(d),
		}, nil
	}

	// One-shot, relative: "30m"
	if d, err := time.ParseDuration(s); err == nil {
		if d <= 0 {
			return nil, fmt.Errorf("执行时间得是将来的某个时刻")
		}
		at := now.Add(d)
		return &ParsedSchedule{
			Kind: types.CronScheduleOnce,
			Expr: at.Format(time.RFC3339),
			Next: at,
		}, nil
	}

	// One-shot, absolute: RFC3339
	if at, err := time.Parse(time.RFC3339, strings.ToUpper(raw)); err == nil {
		if !at.After(now) {
			return nil, fmt.Errorf("%s 已经过去了", at.Format("2006-01-02 15:04"))
		}
		return &ParsedSchedule{
			Kind: types.CronScheduleOnce,
			Expr: at.Format(time.RFC3339),
			Next: at,
		}, nil
	}

	// Cron expression. Accept the 5-field form by prepending a seconds field,
	// because that is what most people paste from crontab.
	expr := raw
	if !strings.HasPrefix(strings.TrimSpace(expr), "@") {
		if n := len(strings.Fields(expr)); n == 5 {
			expr = "0 " + strings.TrimSpace(expr)
		}
	}
	sched, err := cronParser.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("看不懂的执行时间 %q。可以用 cron 表达式（如 \"0 0 9 * * *\" 每天9点）、"+
			"\"every 30m\"、或者 \"2h\" 表示两小时后跑一次", raw)
	}

	next := sched.Next(now)
	if after := sched.Next(next); !after.IsZero() {
		if gap := after.Sub(next); gap < MinCronInterval {
			return nil, fmt.Errorf("这个表达式大约每 %v 触发一次，太频繁了，最快只能每 %v 一次",
				gap.Round(time.Second), MinCronInterval)
		}
	}

	return &ParsedSchedule{Kind: types.CronScheduleCron, Expr: expr, Next: next}, nil
}

// NextAfter returns the next fire time for an already-normalised schedule.
// One-shot jobs have no "next", so they report the zero time.
func NextAfter(kind, expr string, after time.Time) (time.Time, error) {
	if kind == types.CronScheduleOnce {
		return time.Time{}, nil
	}
	sched, err := cronParser.Parse(expr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(after), nil
}
