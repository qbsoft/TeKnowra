package einoruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/runbudget"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"strings"
	"sync"
	"testing"
)

func TestEngineFailsClosedWithoutScope(t *testing.T) {
	if _, err := New(context.Background(), Options{Config: &types.AgentConfig{}, Model: &fixtureChat{}}); err == nil {
		t.Fatal("missing authorization callbacks accepted")
	}
}

type isolatedChat struct{ nativeChat }

func (*isolatedChat) ChatStream(ctx context.Context, messages []chat.Message, _ *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	prompt := messages[len(messages)-1].Content
	reply := `{"queries":["合同政策"]}`
	if strings.Contains(prompt, "审查事实复核") {
		reply = `{"approved":true,"issues":[]}`
	} else if !strings.Contains(prompt, "请生成1至3个") {
		question := strings.Split(strings.Split(prompt, "用户需求：")[1], "\n")[0]
		encoded, _ := json.Marshal(Draft{Summary: question, Citations: []string{"E1"}})
		reply = string(encoded)
	}
	out := make(chan types.StreamResponse, 1)
	out <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: reply, Done: true, FinishReason: "stop"}
	close(out)
	return out, nil
}

func TestConcurrentRunsDoNotShareGraphState(t *testing.T) {
	t.Setenv("TEKNOWRA_AGENT_MAX_CONCURRENT", "8") // This test exercises graph isolation, not admission.
	e, _, _ := reviewFixture(t)
	e.opts.Model = &isolatedChat{}
	e.opts.Events = event.NewEventBus()
	var wg sync.WaitGroup
	ids := sync.Map{}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			question := fmt.Sprintf("synthetic-case-%d", i)
			s, err := e.Execute(context.Background(), question, question, question, nil)
			if err != nil {
				t.Error(err)
				return
			}
			if !strings.Contains(s.FinalAnswer, question) || len(s.RoundSteps) != 4 {
				t.Errorf("cross-run state: %+v", s)
				return
			}
			if _, duplicate := ids.LoadOrStore(s.RoundSteps[0].RunID, true); duplicate {
				t.Error("run ID reused")
			}
		}(i)
	}
	wg.Wait()
}

func TestMalformedPlanAndDraftNeverComplete(t *testing.T) {
	for _, index := range []int{0, 1} {
		e, m, events := reviewFixture(t)
		m.replies[index] = "not JSON"
		if _, err := e.Execute(context.Background(), "s", "m", "审查", nil); err == nil {
			t.Fatal("malformed structure accepted")
		}
		for _, ev := range *events {
			if ev.Type == event.EventAgentComplete {
				t.Fatal("malformed output completed")
			}
		}
	}
}

func TestFailedGraphReturnsPartialStages(t *testing.T) {
	e, m, _ := reviewFixture(t)
	m.replies[1] = "not JSON"
	s, err := e.Execute(context.Background(), "s", "m", "审查", nil)
	if err == nil || s == nil || s.IsComplete || s.FinalAnswer != "" || len(s.RoundSteps) != 3 || s.RoundSteps[2].NodeKey != "analyse" {
		t.Fatalf("partial stages lost: state=%+v err=%v", s, err)
	}
}

func TestCancellationAtOutputCannotComplete(t *testing.T) {
	e, _, events := reviewFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checks := 0
	e.opts.FilterEvidence = func(_ context.Context, _ types.SearchTargets, refs []*types.SearchResult) ([]*types.SearchResult, error) {
		checks++
		if checks == 4 {
			cancel()
		}
		return refs, nil
	}
	if _, err := e.Execute(ctx, "s", "m", "审查", nil); err == nil {
		t.Fatal("cancelled output completed")
	}
	for _, ev := range *events {
		if ev.Type == event.EventAgentComplete || ev.Type == event.EventAgentFinalAnswer {
			t.Fatal("cancelled output published")
		}
	}
}

type sequenceChat struct {
	nativeChat
	calls     int
	replies   []string
	inputs    [][]chat.Message
	usage     *types.TokenUsage
	omitUsage bool
	name      string
	options   []*chat.ChatOptions
}

func (f *sequenceChat) GetModelName() string { return f.name }
func (*isolatedChat) GetModelName() string   { return "synthetic" }
func (f *sequenceChat) ChatStream(ctx context.Context, messages []chat.Message, opts *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	f.options = append(f.options, opts)
	f.inputs = append(f.inputs, messages)
	if f.calls >= len(f.replies) {
		return nil, fmt.Errorf("unexpected model call")
	}
	reply := f.replies[f.calls]
	f.calls++
	out := make(chan types.StreamResponse, 2)
	out <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: reply, Done: true, FinishReason: "stop"}
	usage := f.usage
	if usage == nil {
		usage = &types.TokenUsage{TotalTokens: 3}
	}
	if !f.omitUsage {
		out <- types.StreamResponse{Usage: usage}
	}
	close(out)
	return out, nil
}

func TestV2DeepSeekPinsThinkingWireWithoutIncreasingBudget(t *testing.T) {
	e, m, _ := reviewFixture(t)
	m.name = "deepseek-v4-flash"
	disabled := false
	e.opts.Config.Thinking = &disabled
	if _, err := e.Execute(context.Background(), "s", "m", "review", nil); err != nil {
		t.Fatal(err)
	}
	for _, opts := range m.options {
		if opts.ThinkingControl != "thinking_type" || opts.Thinking == nil || *opts.Thinking || opts.CompletionBudget() != 1024 {
			t.Fatalf("incorrect review options: %+v", opts)
		}
	}
}

func TestFactAuditFailureNeverPublishesReview(t *testing.T) {
	for _, audit := range []string{`{"approved":false,"issues":["币种已明确，不应列缺失"]}`, `{"approved":true,"issues":["前后矛盾"]}`, `{"approved":true}`, `{"approved":true,"issues":null}`, `{}`, `not JSON`} {
		e, m, events := reviewFixture(t)
		m.replies = append(m.replies[:2], audit)
		state, err := e.Execute(context.Background(), "s", "m", "review", nil)
		if err == nil || state == nil || state.IsComplete {
			t.Fatalf("fact audit accepted %s: %v", audit, err)
		}
		for _, ev := range *events {
			if ev.Type == event.EventAgentFinalAnswer || ev.Type == event.EventAgentComplete {
				t.Fatal("failed fact audit published review")
			}
		}
	}
}

type lateSteer struct {
	types.SteerSink
	polls int
}

func (s *lateSteer) PollSteer(context.Context, string, string, int) ([]map[string]interface{}, int, error) {
	s.polls++
	if s.polls == 5 {
		return []map[string]interface{}{{"content": "补充指令"}}, 0, nil
	}
	return nil, 0, nil
}
func TestLateSteerDoesNotSilentlyComplete(t *testing.T) {
	e, _, events := reviewFixture(t)
	e.SetSteerSink(&lateSteer{})
	if _, err := e.Execute(context.Background(), "s", "m", "审查", nil); err == nil {
		t.Fatal("late steer silently ignored")
	}
	for _, ev := range *events {
		if ev.Type == event.EventAgentComplete {
			t.Fatal("late steer emitted completion")
		}
	}
}
func TestConfiguredReviewPromptReachesModel(t *testing.T) {
	e, m, _ := reviewFixture(t)
	e.opts.Config.UseCustomSystemPrompt = true
	e.opts.Config.SystemPrompt = "业务要求：输出风险等级。"
	if _, err := e.Execute(context.Background(), "s", "m", "审查", nil); err != nil {
		t.Fatal(err)
	}
	for _, input := range m.inputs {
		if !strings.Contains(input[0].Content, "输出风险等级") {
			t.Fatal("configured business prompt ignored")
		}
	}
}

func TestReviewPreservesCacheUsage(t *testing.T) {
	e, m, _ := reviewFixture(t)
	m.usage = &types.TokenUsage{PromptTokens: 10, TotalTokens: 12}
	m.usage.SetPromptCacheUsage(6, 0, 4, true)
	s, err := e.Execute(context.Background(), "s", "m", "审查", nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.TurnUsage.CacheReadTokens != 18 || s.TurnUsage.CacheMissTokens != 12 || !s.TurnUsage.CacheReported {
		t.Fatalf("cache accounting lost: %+v", s.TurnUsage)
	}
}

func TestMissingUsageIsNotPublishedAsZero(t *testing.T) {
	e, m, events := reviewFixture(t)
	m.omitUsage = true
	if _, err := e.Execute(context.Background(), "s", "m", "审查", nil); err != nil {
		t.Fatal(err)
	}
	for _, ev := range *events {
		if ev.Type == event.EventAgentComplete {
			usage := ev.Data.(event.AgentCompleteData).Usage
			if u, ok := usage.(*types.TokenUsage); ok && u != nil {
				t.Fatalf("unreported usage published as measured: %+v", u)
			}
		}
	}
}

func TestCitationRepairIsBounded(t *testing.T) {
	for _, valid := range []bool{false, true} {
		t.Run(fmt.Sprint(valid), func(t *testing.T) {
			e, m, events := reviewFixture(t)
			m.replies[1] = `{"summary":"建议补充币种","citations":["unknown"]}`
			repair := m.replies[1]
			if valid {
				repair = `{"summary":"建议补充币种","citations":["E1"]}`
			}
			m.replies = append(m.replies[:2], repair, `{"approved":true,"issues":[]}`)
			_, err := e.Execute(context.Background(), "s", "m", "审查", nil)
			wantCalls := 3
			if valid {
				wantCalls = 4
			}
			if (err == nil) != valid || m.calls != wantCalls {
				t.Fatalf("err=%v calls=%d", err, m.calls)
			}
			for _, ev := range *events {
				if !valid && (ev.Type == event.EventAgentComplete || ev.Type == event.EventAgentFinalAnswer) {
					t.Fatal("invalid citation completed")
				}
			}
		})
	}
}
func reviewFixture(t *testing.T) (*Engine, *sequenceChat, *[]event.Event) {
	t.Helper()
	model := &sequenceChat{replies: []string{`{"queries":["合同付款政策"]}`, `{"summary":"未约定币种，应补充确认。","citations":["E1"]}`, `{"approved":true,"issues":[]}`}}
	scope := types.SearchTargets{&types.SearchTarget{Type: types.SearchTargetTypeKnowledgeBase, KnowledgeBaseID: "kb", TenantID: 10000}}
	refs := []*types.SearchResult{&types.SearchResult{ID: "chunk", KnowledgeID: "doc", KnowledgeBaseID: "kb", Content: "总价100，币种未约定", KnowledgeTitle: "合成合同"}}
	bus := event.NewEventBus()
	events := []event.Event{}
	for _, kind := range []event.EventType{event.EventAgentThought, event.EventAgentReferences, event.EventAgentFinalAnswer, event.EventAgentComplete} {
		bus.On(kind, func(_ context.Context, e event.Event) error { events = append(events, e); return nil })
	}
	eng, err := New(context.Background(), Options{Config: &types.AgentConfig{SearchTargets: scope}, Model: model, Events: bus,
		Revalidate: func(context.Context) (types.SearchTargets, error) { return scope, nil },
		Retrieve:   func(context.Context, types.SearchTargets, []string) ([]*types.SearchResult, error) { return refs, nil },
		FilterEvidence: func(_ context.Context, _ types.SearchTargets, v []*types.SearchResult) ([]*types.SearchResult, error) {
			return v, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	return eng.(*Engine), model, &events
}
func TestCompleteGraphPreservesStagesAndCitations(t *testing.T) {
	e, m, events := reviewFixture(t)
	state, err := e.Execute(context.Background(), "session", "message", "审查合同", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !state.IsComplete || len(state.RoundSteps) != 4 || m.calls != 3 || state.TurnUsage.TotalTokens != 9 || !strings.Contains(state.FinalAnswer, `chunk_id="chunk"`) {
		t.Fatalf("state=%+v calls=%d", state, m.calls)
	}
	complete := 0
	for _, ev := range *events {
		if ev.Type == event.EventAgentComplete {
			complete++
		}
		if ev.Type == event.EventAgentThought {
			v := ev.Data.(event.AgentThoughtData)
			if v.RunID == "" || v.NodeKey == "" || v.DefinitionVersion != DefinitionVersion {
				t.Fatal("node metadata lost")
			}
		}
	}
	if complete != 1 {
		t.Fatalf("completion count=%d", complete)
	}
}
func TestEvidenceChangedBeforeOutputFailsClosed(t *testing.T) {
	e, _, events := reviewFixture(t)
	checks := 0
	e.opts.FilterEvidence = func(_ context.Context, _ types.SearchTargets, v []*types.SearchResult) ([]*types.SearchResult, error) {
		checks++
		if checks == 4 {
			copyRef := *v[0]
			copyRef.Content = "合同已更新"
			return []*types.SearchResult{&copyRef}, nil
		}
		return v, nil
	}
	_, err := e.Execute(context.Background(), "s", "m", "审查", nil)
	if err == nil {
		t.Fatal("changed evidence was published as successful review")
	}
	for _, ev := range *events {
		if ev.Type == event.EventAgentComplete || ev.Type == event.EventAgentFinalAnswer {
			t.Fatal("failed review emitted success")
		}
	}
}
func TestRevokedBeforeModelStopsGraph(t *testing.T) {
	e, m, events := reviewFixture(t)
	checks := 0
	e.opts.Revalidate = func(context.Context) (types.SearchTargets, error) {
		checks++
		if checks >= 2 {
			return nil, fmt.Errorf("revoked")
		}
		return e.opts.Config.SearchTargets, nil
	}
	if _, err := e.Execute(context.Background(), "s", "m", "审查", nil); err == nil {
		t.Fatal("revocation ignored")
	}
	if m.calls != 1 {
		t.Fatalf("evidence sent to model after revocation: %d", m.calls)
	}
	for _, ev := range *events {
		if ev.Type == event.EventAgentComplete {
			t.Fatal("revoked run complete")
		}
	}
}
func TestCancelledGraphNeverCompletes(t *testing.T) {
	e, m, events := reviewFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	e.opts.Retrieve = func(context.Context, types.SearchTargets, []string) ([]*types.SearchResult, error) {
		cancel()
		return nil, context.Canceled
	}
	if _, err := e.Execute(ctx, "s", "m", "审查", nil); err == nil || !strings.Contains(err.Error(), "任务已停止") {
		t.Fatalf("cancellation reason missing: %v", err)
	}
	if m.calls != 1 {
		t.Fatal("model called after cancellation")
	}
	for _, ev := range *events {
		if ev.Type == event.EventAgentComplete {
			t.Fatal("cancelled run complete")
		}
	}
}

func TestGraphPreservesAuthorizationWaitBudgetCause(t *testing.T) {
	e, _, _ := reviewFixture(t)
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	e.opts.Retrieve = func(context.Context, types.SearchTargets, []string) ([]*types.SearchResult, error) {
		cancel(runbudget.ErrWaitExceeded)
		return nil, context.Canceled
	}
	_, err := e.Execute(ctx, "s", "m", "审查", nil)
	if err == nil || err.Error() != runbudget.ErrWaitExceeded.Error() {
		t.Fatalf("authorization timeout presented as generic cancellation: %v", err)
	}
}
func TestCitationValidatorRejectsUnknown(t *testing.T) {
	refs := []*types.SearchResult{{ID: "allowed", KnowledgeID: "doc", KnowledgeBaseID: "kb"}}
	if _, err := RenderValidatedAnswer(Draft{Summary: "text", Citations: []string{"foreign"}}, refs); err == nil {
		t.Fatal("forged citation accepted")
	}
	if _, err := RenderValidatedAnswer(Draft{Summary: `<kb chunk_id="foreign"/>`, Citations: []string{"allowed"}}, refs); err == nil {
		t.Fatal("forged inline citation accepted")
	}
	text, err := RenderValidatedAnswer(Draft{Summary: "text", Citations: []string{"allowed"}}, refs)
	if err != nil || text == "" {
		t.Fatal(err)
	}
}
func TestEngineRejectsSteer(t *testing.T) {
	// Must fail explicitly rather than silently ignore an interface setter.
	e := &Engine{steerUnsupported: true}
	if _, err := e.Execute(context.Background(), "s", "m", "query", nil); err == nil {
		t.Fatal("steer ignored")
	}
}
