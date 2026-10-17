package einoruntime

import (
	"context"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"strings"
	"testing"
)

func TestThirdDefinitionExecutesThroughCommonEngine(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		t.Run(fmt.Sprintf("revoked=%t", revoke), func(t *testing.T) {
			e, m, events := reviewFixture(t)
			o := e.opts
			o.Definition = "knowledge-summary-v1"
			m.replies = []string{`{"summary":"总价100，币种未约定","citations":["E1"]}`, `{"approved":true,"issues":[]}`}
			if revoke {
				original := o.FilterEvidence
				o.FilterEvidence = func(ctx context.Context, scope types.SearchTargets, refs []*types.SearchResult) ([]*types.SearchResult, error) {
					if m.calls > 0 {
						return nil, nil
					}
					return original(ctx, scope, refs)
				}
			}
			engine, err := New(context.Background(), o)
			if err != nil {
				t.Fatalf("third definition rejected: %v", err)
			}
			state, err := engine.Execute(context.Background(), "summary-session", "summary-message", "摘要", nil)
			if (err != nil) != revoke {
				t.Fatalf("revoked=%t err=%v", revoke, err)
			}
			if !revoke && (!state.IsComplete || !strings.Contains(state.FinalAnswer, "总价100") || len(state.KnowledgeRefs) != 1) {
				t.Fatalf("incomplete summary: %+v", state)
			}
			completed, references := false, false
			for _, ev := range *events {
				if revoke && (ev.Type == event.EventAgentFinalAnswer || ev.Type == event.EventAgentComplete) {
					t.Fatal("revoked output published")
				}
				if ev.Type == event.EventAgentComplete {
					completed = true
				}
				if ev.Type == event.EventAgentReferences {
					references = true
				}
			}
			if !revoke && (!completed || !references) {
				t.Fatal("common references/completion event missing")
			}
		})
	}
}

func TestSummaryRejectsUnsupportedFactsDespiteValidCitation(t *testing.T) {
	for _, audit := range []string{`{"approved":false,"issues":["金额与付款事实无依据"]}`, `{"approved":true,"issues":["矛盾"]}`, `{}`, `not-json`} {
		e, m, events := reviewFixture(t)
		o := e.opts
		o.Definition = "knowledge-summary-v1"
		m.replies = []string{`{"summary":"总价100万元且已付款","citations":["E1"]}`, audit}
		engine, err := New(context.Background(), o)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Execute(context.Background(), "s", "m", "摘要", nil); err == nil {
			t.Fatal("unsupported summary with valid citation published")
		}
		for _, ev := range *events {
			if ev.Type == event.EventAgentFinalAnswer || ev.Type == event.EventAgentComplete {
				t.Fatal("failed fact audit published output")
			}
		}
	}
}

func TestEveryPublishedDefinitionHasBuilder(t *testing.T) {
	for _, id := range einodef.IDs() {
		p, ok := publications[id]
		if !ok || p.Build == nil || p.SystemPrompt == "" {
			t.Fatalf("published definition lacks implementation: %s", id)
		}
	}
	for id := range publications {
		if _, ok := einodef.Lookup(id); !ok {
			t.Fatalf("builder lacks capability manifest: %s", id)
		}
	}
}

func TestUnknownDefinitionNeverFallsBack(t *testing.T) {
	e, _, _ := reviewFixture(t)
	o := e.opts
	o.Definition = "unpublished-v1"
	if _, err := New(context.Background(), o); err == nil {
		t.Fatal("unknown definition used default implementation")
	}
}
