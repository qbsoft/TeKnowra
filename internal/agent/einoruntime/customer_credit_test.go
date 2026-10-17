package einoruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestCreditDefinitionUsesReadOnlyCapabilities(t *testing.T) {
	for _, scenario := range []string{"missing_credit_id", "success", "revoked", "ambiguous", "numeric_id", "forged_name"} {
		t.Run(scenario, func(t *testing.T) {
			var calls []Invocation
			var published bool
			bus := event.NewEventBus()
			bus.On(event.EventAgentFinalAnswer, func(_ context.Context, _ event.Event) error { published = true; return nil })
			bus.On(event.EventAgentComplete, func(_ context.Context, ev event.Event) error {
				complete := ev.Data.(event.AgentCompleteData)
				if _, ok := complete.Extra["wait_ms"]; !ok {
					t.Error("complete event is missing cumulative wait time")
				}
				if _, ok := complete.Extra["active_ms"]; !ok {
					t.Error("complete event is missing active time")
				}
				return nil
			})
			model := &sequenceChat{replies: []string{`{"customer_name":"甲客户"}`}}
			if scenario == "forged_name" {
				model.replies[0] = `{"customer_name":"乙客户"}`
			}
			o := Options{Config: &types.AgentConfig{SearchTargets: types.SearchTargets{&types.SearchTarget{KnowledgeBaseID: "unused-resolved-scope"}}}, Model: model, Events: bus, Definition: CreditDefinition,
				Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, nil },
				Retrieve: func(context.Context, types.SearchTargets, []string) ([]*types.SearchResult, error) {
					t.Fatal("credit agent queried knowledge")
					return nil, nil
				},
				FilterEvidence: func(context.Context, types.SearchTargets, []*types.SearchResult) ([]*types.SearchResult, error) {
					return nil, nil
				},
				CallReadOnly: func(ctx context.Context, in Invocation) (map[string]any, error) {
					calls = append(calls, in)
					if in.SessionID != "session" || in.MessageID != "message" || in.RunID == "" || in.ToolCallID == "" {
						t.Fatal("missing runtime identity")
					}
					switch in.Tool {
					case "find_customer":
						id := any("2073967557021581313")
						if scenario == "numeric_id" {
							id = json.Number("2073967557021581313")
						}
						rows := []any{map[string]any{"id": id, "name": "甲客户"}}
						if scenario == "ambiguous" {
							rows = append(rows, map[string]any{"id": "9", "name": "甲客户分公司"})
						}
						return map[string]any{"customers": rows, "total": json.Number(fmt.Sprint(len(rows)))}, nil
					case "check_customer_access":
						if scenario == "revoked" && len(calls) > 3 {
							return nil, fmt.Errorf("revoked")
						}
						return map[string]any{"customer_id": "2073967557021581313", "readable": true}, nil
					case "get_customer_credit_control":
						if in.Args["customer_id"] != "2073967557021581313" {
							t.Fatal("ID was changed")
						}
						out := map[string]any{"customerId": "2073967557021581313", "creditAvailable": json.Number("100.01"), "riskGrade": "A"}
						if scenario == "missing_credit_id" {
							delete(out, "customerId")
						}
						return out, nil
					default:
						t.Fatalf("unexpected tool %s", in.Tool)
						return nil, nil
					}
				}}
			engine, err := New(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			state, err := engine.Execute(context.Background(), "session", "message", "查询甲客户的信用额度", nil)
			if scenario == "success" {
				if err != nil || !published || !state.IsComplete || !strings.Contains(state.FinalAnswer, "100.01") {
					t.Fatalf("failed: %v %+v", err, state)
				}
				if len(calls) != 4 {
					t.Fatalf("expected lookup,check,credit,finalcheck: %v", calls)
				}
				var recorded []types.ToolCall
				for _, step := range state.RoundSteps {
					recorded = append(recorded, step.ToolCalls...)
				}
				if len(recorded) != 4 {
					t.Fatalf("tool history not persisted: %d", len(recorded))
				}
				for i, record := range recorded {
					if record.ID != calls[i].ToolCallID || record.Result == nil || !record.Result.Success {
						t.Fatal("history differs from live execution")
					}
				}
			} else if err == nil || published {
				t.Fatalf("unsafe output for %s: %v", scenario, err)
			}
		})
	}
}
