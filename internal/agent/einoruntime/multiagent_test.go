package einoruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/cloudwego/eino/schema"
	"strings"
	"testing"
)

type delegationFixture struct {
	nativeChat
	experts        []string
	calls          int
	toolInputs     [][]chat.Message
	businessInputs [][]chat.Message
	badTool        string
	badArgs        string
	repeat         bool
}

func (*delegationFixture) GetModelName() string { return "synthetic" }
func (f *delegationFixture) ChatStream(ctx context.Context, ms []chat.Message, opts *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	f.calls++
	response := types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Done: true, FinishReason: "stop", Content: "完成"}
	if len(opts.Tools) > 0 {
		f.toolInputs = append(f.toolInputs, ms)
		completed := map[string]bool{}
		for _, m := range ms {
			for _, tc := range m.ToolCalls {
				completed[tc.Function.Name] = true
			}
		}
		name := ""
		if opts.Tools[0].Function.Name == "execute_contract" || opts.Tools[0].Function.Name == "execute_credit" {
			if !completed[opts.Tools[0].Function.Name] {
				name = opts.Tools[0].Function.Name
			}
		} else {
			for _, expert := range f.experts {
				if !completed[expert] {
					name = expert
					break
				}
			}
		}
		if name != "" {
			response.Content = ""
			response.FinishReason = "tool_calls"
			response.ToolCalls = []types.LLMToolCall{{ID: fmt.Sprintf("call-%d", f.calls), Type: "function", Function: types.FunctionCall{Name: name, Arguments: `{}`}}}
		}
		if f.repeat && opts.Tools[0].Function.Name == "contract_expert" {
			response.Content = ""
			response.FinishReason = "tool_calls"
			response.ToolCalls = []types.LLMToolCall{{ID: fmt.Sprintf("call-%d", f.calls), Type: "function", Function: types.FunctionCall{Name: "contract_expert", Arguments: `{}`}}}
		}
		if f.badTool != "" {
			response.ToolCalls = []types.LLMToolCall{{ID: "invalid", Type: "function", Function: types.FunctionCall{Name: f.badTool, Arguments: `{}`}}}
			response.FinishReason = "tool_calls"
		}
		if f.badArgs != "" && len(response.ToolCalls) > 0 {
			response.ToolCalls[0].Function.Arguments = f.badArgs
		}
	} else {
		f.businessInputs = append(f.businessInputs, ms)
		prompt := ms[len(ms)-1].Content
		switch {
		case strings.Contains(prompt, "请生成1至3个"):
			response.Content = `{"queries":["合同"]}`
		case strings.Contains(prompt, "审查事实复核"):
			response.Content = `{"approved":true,"issues":[]}`
		case strings.Contains(prompt, "只提取"):
			response.Content = `{"customer_name":"甲客户"}`
		default:
			response.Content = `{"summary":"合同合成金额100元","citations":["E1"]}`
		}
	}
	out := make(chan types.StreamResponse, 2)
	out <- response
	out <- types.StreamResponse{Usage: &types.TokenUsage{TotalTokens: 3}}
	close(out)
	return out, nil
}

func TestContractExpertKeepsOriginalQueryWithinItsResponsibility(t *testing.T) {
	base, _, _ := reviewFixture(t)
	fake := &delegationFixture{experts: []string{"contract_expert"}}
	o := base.opts
	o.Definition = "business-review-multiagent-v1"
	o.Model = fake
	o.CallReadOnly = func(context.Context, Invocation) (map[string]any, error) {
		t.Fatal("contract expert must not call CRM")
		return nil, nil
	}
	e, err := New(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	query := "审查合同TEST-A-001，并查询甲客户信用；不推断两者关联"
	state, err := e.Execute(context.Background(), "s", "m", query, nil)
	if err != nil || !state.IsComplete {
		t.Fatalf("execution failed: %v", err)
	}
	if len(fake.businessInputs) != 3 {
		t.Fatalf("expected plan, analysis and audit: %d", len(fake.businessInputs))
	}
	for _, messages := range fake.businessInputs {
		if !strings.Contains(messages[0].Content, "客户信用由信用专家独立处理") {
			t.Fatal("contract business model lacks expert responsibility; may report sibling task as missing")
		}
		if !strings.Contains(messages[len(messages)-1].Content, query) {
			t.Fatal("original user query changed")
		}
	}
	// A standalone review must keep the original publication prompt.
	old, model, _ := reviewFixture(t)
	if _, err := old.Execute(context.Background(), "s", "m", "合同", nil); err != nil {
		t.Fatal(err)
	}
	for _, messages := range model.inputs {
		if strings.Contains(messages[0].Content, "客户信用由信用专家独立处理") {
			t.Fatal("expert scope leaked into standalone review")
		}
	}
}

func TestMultiAgentDelegatesAndSharesProductState(t *testing.T) {
	for _, experts := range [][]string{{"contract_expert"}, {"credit_expert"}, {"contract_expert", "credit_expert"}} {
		t.Run(strings.Join(experts, "+"), func(t *testing.T) {
			base, _, _ := reviewFixture(t)
			fake := &delegationFixture{experts: experts}
			o := base.opts
			o.Definition = "business-review-multiagent-v1"
			o.Model = fake
			// Match the real host filter: it materializes a non-nil empty list
			// when no expert used knowledge. The research identity filter did not.
			originalFilter := o.FilterEvidence
			o.FilterEvidence = func(ctx context.Context, scope types.SearchTargets, refs []*types.SearchResult) ([]*types.SearchResult, error) {
				if len(refs) == 0 {
					return []*types.SearchResult{}, nil
				}
				return originalFilter(ctx, scope, refs)
			}
			var calls []Invocation
			o.CallReadOnly = func(ctx context.Context, in Invocation) (map[string]any, error) {
				calls = append(calls, in)
				switch in.Tool {
				case "find_customer":
					return map[string]any{"customers": []any{map[string]any{"id": "2073967557021581313", "name": "甲客户"}}, "total": json.Number("1")}, nil
				case "check_customer_access":
					return map[string]any{"customer_id": "2073967557021581313", "readable": true}, nil
				case "get_customer_credit_control":
					return map[string]any{"customerId": "2073967557021581313", "creditAvailable": json.Number("100.01")}, nil
				}
				return nil, fmt.Errorf("unexpected tool")
			}
			engine, err := New(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			state, err := engine.Execute(context.Background(), "session", "message", "审查合同并查询甲客户信用", nil)
			if err != nil || !state.IsComplete {
				t.Fatalf("failed: %v %+v", err, state)
			}
			contract, credit := false, false
			for _, x := range experts {
				contract = contract || x == "contract_expert"
				credit = credit || x == "credit_expert"
			}
			if strings.Contains(state.FinalAnswer, "合成金额100元") != contract || strings.Contains(state.FinalAnswer, "100.01") != credit {
				t.Fatal("expert results do not match actual delegation")
			}
			if credit && len(calls) != 4 || !credit && len(calls) != 0 {
				t.Fatalf("CRM isolation: %d", len(calls))
			}
			runID := ""
			seen := map[string]bool{}
			for _, step := range state.RoundSteps {
				if runID == "" {
					runID = step.RunID
				}
				if step.RunID != runID || step.DefinitionVersion != o.Definition {
					t.Fatal("expert task identity lost")
				}
				seen[step.NodeKey] = true
				for _, call := range step.ToolCalls {
					if call.Result == nil || !call.Result.Success {
						t.Fatalf("successful run has unfinished call: %+v", call)
					}
				}
			}
			if !seen["supervisor"] || contract && !seen["contract_expert/retrieve"] || credit && !seen["credit_expert/credit"] {
				t.Fatalf("expert steps missing: %v", seen)
			}
			if state.TurnUsage.TotalTokens != fake.calls*3 {
				t.Fatal("model usage not shared")
			}
			for _, ms := range fake.toolInputs {
				for _, m := range ms {
					if strings.Contains(m.Content, "100.01") {
						t.Fatal("CRM business data transmitted to delegating model")
					}
				}
			}
		})
	}
}

func TestMultiAgentCannotCompleteWithoutExpert(t *testing.T) {
	base, _, events := reviewFixture(t)
	o := base.opts
	o.Definition = "business-review-multiagent-v1"
	o.Model = &delegationFixture{}
	o.CallReadOnly = func(context.Context, Invocation) (map[string]any, error) {
		t.Fatal("unexpected CRM call")
		return nil, nil
	}
	e, err := New(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Execute(context.Background(), "s", "m", "问题", nil)
	if err == nil {
		t.Fatal("fabricated completion accepted")
	}
	for _, ev := range *events {
		if ev.Type == event.EventAgentComplete {
			t.Fatal("failure completed")
		}
	}
}

func TestMultiAgentRejectsInvalidDelegationAndFailedExpert(t *testing.T) {
	for _, scenario := range []string{"unknown_tool", "resource_args", "repeat", "CRM_denied", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			base, _, events := reviewFixture(t)
			o := base.opts
			o.Definition = "business-review-multiagent-v1"
			f := &delegationFixture{experts: []string{"contract_expert"}}
			if scenario == "unknown_tool" {
				f.badTool = "write"
			}
			if scenario == "resource_args" {
				f.badArgs = `{"customer_id":"9"}`
			}
			if scenario == "repeat" {
				f.repeat = true
			}
			if scenario == "CRM_denied" {
				f.experts = []string{"credit_expert"}
			}
			o.Model = f
			o.CallReadOnly = func(context.Context, Invocation) (map[string]any, error) { return nil, fmt.Errorf("permission denied") }
			e, err := New(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "cancelled" {
				cancel()
			}
			state, err := e.Execute(ctx, "s", "m", "审查合同并查询甲客户", nil)
			if err == nil || state != nil && state.IsComplete {
				t.Fatal("unsafe run completed")
			}
			for _, ev := range *events {
				if ev.Type == event.EventAgentComplete || ev.Type == event.EventAgentFinalAnswer {
					t.Fatal("failed expert published final result")
				}
			}
		})
	}
}
func TestADKModelUsesCumulativeTaskCallLimit(t *testing.T) {
	base, _, _ := reviewFixture(t)
	f := &delegationFixture{}
	base.opts.Model = f
	r := &run{engine: base, state: &types.AgentState{}, calls: 16, usageComplete: true, expertResults: map[string]string{}}
	ctx := context.WithValue(context.Background(), runKey{}, r)
	_, err := (&taskChatModel{run: r, agent: "test"}).Generate(ctx, []*schema.Message{schema.UserMessage("test")})
	if err == nil || f.calls != 0 {
		t.Fatal("subagent reset task call budget")
	}
}
