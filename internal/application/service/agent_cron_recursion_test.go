package service

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
)

func agentWithTools(names ...string) *types.CustomAgent {
	a := &types.CustomAgent{ID: "agent-1"}
	a.Config.AllowedTools = names
	return a
}

func has(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}

// A scheduled run must not be able to schedule more runs: one job whose prompt
// drifts into "and set this up to repeat" would spawn two, then four, and every
// individual job would look legitimate.
func TestWithoutSelfScheduling_StripsCronjob(t *testing.T) {
	agent := agentWithTools(tools.ToolThinking, tools.ToolCronjob, tools.ToolKnowledgeSearch)

	got := withoutSelfScheduling(agent)

	if has(got.Config.AllowedTools, tools.ToolCronjob) {
		t.Fatal("cronjob survived into a cron run — jobs can create jobs")
	}
	for _, keep := range []string{tools.ToolThinking, tools.ToolKnowledgeSearch} {
		if !has(got.Config.AllowedTools, keep) {
			t.Errorf("%s was dropped; only cronjob should be removed", keep)
		}
	}
}

// The agent comes from a service that may cache or share it. Stripping in
// place would quietly disable scheduling for the interactive users of that
// same agent.
func TestWithoutSelfScheduling_DoesNotMutateOriginal(t *testing.T) {
	agent := agentWithTools(tools.ToolThinking, tools.ToolCronjob)

	_ = withoutSelfScheduling(agent)

	if !has(agent.Config.AllowedTools, tools.ToolCronjob) {
		t.Fatal("the shared agent was mutated — interactive users just lost the tool")
	}
}

func TestWithoutSelfScheduling_NoopWhenAbsent(t *testing.T) {
	agent := agentWithTools(tools.ToolThinking)

	got := withoutSelfScheduling(agent)

	if len(got.Config.AllowedTools) != 1 {
		t.Errorf("tools = %v, want them untouched", got.Config.AllowedTools)
	}
}
