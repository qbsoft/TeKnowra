package service

import (
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestFilterAskedSuggestionItems(t *testing.T) {
	items := types.SuggestionItems{
		{Text: "2026年9月相比2026年8月的销售额变化是多少？"},
		{Text: " 2026年9月相比2026年8月的销售额变化是多少? "},
		{Text: "减少最多客户的两期订单有哪些？"},
		{Text: "九月销售额是多少？"},
	}
	got := filterAskedSuggestionItems(items, []string{"2026年9月相比2026年8月的销售额变化是多少？", "九月销售额是多少"})
	if len(got) != 1 || got[0].Text != "减少最多客户的两期订单有哪些？" {
		t.Fatalf("asked questions leaked: %#v", got)
	}
}
