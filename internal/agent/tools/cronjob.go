package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// listOutputExcerpt bounds how much of a past run's output goes back into the
// model's context. Enough to answer "what did it find yesterday", not enough
// for one chatty job to crowd out the conversation.
const listOutputExcerpt = 400

var cronjobTool = BaseTool{
	name: ToolCronjob,
	description: `管理用户的定时任务：到点自动执行一段指令。

## 什么时候用
用户说「每天早上帮我看看…」「每周一提醒我…」「一小时后…」这类带时间的重复或延迟需求时。
用户问「我有哪些定时任务」「把那个任务停掉」时，也用这个工具。

## action 说明
- create  新建。必须给 schedule 和 prompt
- list    列出用户自己的任务（含上次执行结果摘要）
- update  改时间或内容，要给 job_id
- pause / resume  暂停 / 恢复，要给 job_id
- run     立刻跑一次，不影响原排程，要给 job_id
- remove  删除，要给 job_id

## schedule 怎么填 —— 重要
**你负责把用户的中文说法换算成 cron 表达式**，不要原样传「每天早上9点」，那样会被拒绝。

	每天早上9点      → "0 0 9 * * *"     （六段：秒 分 时 日 月 周）
	每周一上午10点   → "0 0 10 * * 1"
	每半小时         → "every 30m"
	两小时后跑一次   → "2h"
	某个具体时刻     → RFC3339，如 "2026-09-01T08:30:00Z"

**换算不确定就先问用户**，别猜。时间点猜错了，用户要等到任务在错误的时间触发才会发现。

最短间隔 5 分钟，比这更频繁会被拒绝。

## prompt 怎么填
写清楚到点之后要做什么，像交代给一个当时不在场的人。
它会在一个全新会话里执行，看不到现在这段对话，所以别写「照上面说的做」。

## 用户要确认的事
新建之后把「什么时间、做什么、下次什么时候跑」复述给用户，让他能当场纠正。`,
	schema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["create", "list", "update", "pause", "resume", "run", "remove"],
				"description": "要做的操作"
			},
			"job_id": {
				"type": "string",
				"description": "任务 ID。create 和 list 之外的操作都必须给"
			},
			"schedule": {
				"type": "string",
				"description": "执行时间。cron 表达式(如 \"0 0 9 * * *\")、\"every 30m\"、\"2h\"、或 RFC3339 时刻。不要传中文"
			},
			"prompt": {
				"type": "string",
				"description": "到点之后要执行的指令，写成不依赖当前对话也能看懂的样子"
			},
			"name": {
				"type": "string",
				"description": "任务名，便于用户识别。不给则从 prompt 自动截取"
			},
			"repeat": {
				"type": "integer",
				"description": "只跑指定次数后自动停用。不给则一直跑。失败不计入次数"
			}
		},
		"required": ["action"]
	}`),
}

// CronjobTool lets an agent manage the caller's scheduled tasks.
type CronjobTool struct {
	BaseTool
	manager interfaces.AgentCronManager
	// agentID records which agent a new job should run as.
	agentID string
}

// NewCronjobTool creates the cronjob tool.
func NewCronjobTool(manager interfaces.AgentCronManager, agentID string) *CronjobTool {
	return &CronjobTool{BaseTool: cronjobTool, manager: manager, agentID: agentID}
}

type cronjobInput struct {
	Action   string `json:"action"`
	JobID    string `json:"job_id"`
	Schedule string `json:"schedule"`
	Prompt   string `json:"prompt"`
	Name     string `json:"name"`
	Repeat   int    `json:"repeat"`
}

// Execute runs one cronjob action.
func (t *CronjobTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var in cronjobInput
	if err := json.Unmarshal(args, &in); err != nil {
		return failure(fmt.Sprintf("参数解析失败：%v", err)), nil
	}

	// Every action but create/list needs an id, and saying so plainly saves a
	// round-trip of the model guessing.
	if in.Action != "create" && in.Action != "list" && in.JobID == "" {
		return failure("这个操作需要 job_id，可以先用 action=list 查一下"), nil
	}

	switch in.Action {
	case "create":
		job, err := t.manager.Create(ctx, interfaces.CreateJobInput{
			Schedule: in.Schedule,
			Prompt:   in.Prompt,
			Name:     in.Name,
			Repeat:   in.Repeat,
			AgentID:  t.agentID,
		})
		if err != nil {
			// The service writes its errors for the end user, so pass them
			// through rather than wrapping them in tool-speak.
			return failure(err.Error()), nil
		}
		return success(fmt.Sprintf("定时任务「%s」建好了。%s\n任务 ID：%s",
			job.Name, describeSchedule(job), job.ID), jobData(job)), nil

	case "list":
		jobs, err := t.manager.List(ctx)
		if err != nil {
			return failure(err.Error()), nil
		}
		if len(jobs) == 0 {
			return success("你还没有定时任务。", nil), nil
		}
		return success(renderJobList(jobs), map[string]interface{}{"count": len(jobs)}), nil

	case "update":
		job, err := t.manager.Update(ctx, in.JobID, interfaces.UpdateJobInput{
			Schedule: in.Schedule,
			Prompt:   in.Prompt,
			Name:     in.Name,
		})
		if err != nil {
			return failure(err.Error()), nil
		}
		return success(fmt.Sprintf("「%s」改好了。%s", job.Name, describeSchedule(job)), jobData(job)), nil

	case "pause":
		job, err := t.manager.SetPaused(ctx, in.JobID, true)
		if err != nil {
			return failure(err.Error()), nil
		}
		return success(fmt.Sprintf("「%s」已暂停，不会再触发，直到你恢复它。", job.Name), jobData(job)), nil

	case "resume":
		job, err := t.manager.SetPaused(ctx, in.JobID, false)
		if err != nil {
			return failure(err.Error()), nil
		}
		return success(fmt.Sprintf("「%s」已恢复。%s", job.Name, describeSchedule(job)), jobData(job)), nil

	case "run":
		job, err := t.manager.RunNow(ctx, in.JobID)
		if err != nil {
			return failure(err.Error()), nil
		}
		return success(fmt.Sprintf("「%s」已经开始跑了，在后台执行，结果稍后可以用 action=list 查看。"+
			"原来的排程不受影响。", job.Name), jobData(job)), nil

	case "remove":
		job, err := t.manager.Remove(ctx, in.JobID)
		if err != nil {
			return failure(err.Error()), nil
		}
		return success(fmt.Sprintf("「%s」已删除。", job.Name), jobData(job)), nil

	default:
		return failure(fmt.Sprintf("不认识的操作 %q，可用：create / list / update / pause / resume / run / remove",
			in.Action)), nil
	}
}

func success(output string, data map[string]interface{}) *types.ToolResult {
	return &types.ToolResult{Success: true, Output: output, Data: data}
}

// failure returns a result the model can act on rather than an error.
//
// A tool error aborts the step; a failed result lets the model read why and
// try again — which for "that schedule is too frequent" or "I need a job_id"
// is exactly what should happen.
func failure(msg string) *types.ToolResult {
	return &types.ToolResult{Success: false, Output: msg, Error: msg}
}

func jobData(job *types.AgentCronJob) map[string]interface{} {
	return map[string]interface{}{
		"job_id":      job.ID,
		"name":        job.Name,
		"schedule":    job.ScheduleExpr,
		"next_run_at": formatTime(job.NextRunAt),
		"paused":      job.Paused,
		"enabled":     job.Enabled,
	}
}

func describeSchedule(job *types.AgentCronJob) string {
	if job.Paused {
		return "当前是暂停状态。"
	}
	if job.NextRunAt == nil {
		return "已经没有后续执行了。"
	}
	s := fmt.Sprintf("下次执行：%s。", job.NextRunAt.Local().Format("2006-01-02 15:04"))
	if job.RepeatLeft != nil {
		s += fmt.Sprintf("还会执行 %d 次。", *job.RepeatLeft)
	}
	return s
}

// renderJobList formats the jobs as text for the model to relay.
//
// Past output is included but excerpted: the user asking "what did that job
// find" should get an answer without a single verbose job filling the window.
func renderJobList(jobs []*types.AgentCronJob) string {
	var b strings.Builder
	fmt.Fprintf(&b, "你有 %d 个定时任务：\n", len(jobs))

	for i, job := range jobs {
		state := "运行中"
		switch {
		case job.Paused:
			state = "已暂停"
		case !job.Enabled:
			state = "已停用"
		}

		fmt.Fprintf(&b, "\n%d. 「%s」[%s]\n   ID：%s\n   排程：%s　下次：%s\n",
			i+1, job.Name, state, job.ID, job.ScheduleExpr, formatTime(job.NextRunAt))

		if job.LastRunAt != nil {
			outcome := "成功"
			if job.LastStatus == types.CronStatusFailed {
				outcome = "失败"
			}
			fmt.Fprintf(&b, "   上次：%s %s", job.LastRunAt.Local().Format("01-02 15:04"), outcome)
			if job.FailureStreak > 1 {
				fmt.Fprintf(&b, "（已连续失败 %d 次）", job.FailureStreak)
			}
			b.WriteString("\n")

			if job.LastError != "" {
				fmt.Fprintf(&b, "   错误：%s\n", excerpt(job.LastError, 200))
			}
			if job.LastOutput != "" {
				fmt.Fprintf(&b, "   结果：%s\n", excerpt(job.LastOutput, listOutputExcerpt))
			}
		}
	}
	return b.String()
}

func excerpt(s string, limit int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…（略）"
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "无"
	}
	return t.Local().Format("2006-01-02 15:04")
}
