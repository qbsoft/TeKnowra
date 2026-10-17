package langflowruntime

import (
	"github.com/Tencent/WeKnora/internal/models/chat"
	"reflect"
	"strings"
	"testing"
)

func TestHistoryKeepsCompletedTextTurnsWithoutToolOrReasoningReplay(t *testing.T) {
	input := []chat.Message{{Role: "system", Content: "not a historical instruction"}, {Role: "user", Content: "项目代号松石"}, {Role: "assistant", Content: "tool thought", ToolCalls: []chat.ToolCall{{ID: "old"}}}, {Role: "tool", Content: "old confidential result", ToolCallID: "old"}, {Role: "assistant", Content: "收到松石", ReasoningContent: "private reasoning"}, {Role: "user", Content: "incomplete"}}
	got, err := conversationHistory(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []chat.Message{{Role: "user", Content: "项目代号松石"}, {Role: "assistant", Content: "收到松石"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unsafe or incomplete history replay: %#v", got)
	}
	input[1].Content = "changed"
	if got[0].Content != "项目代号松石" {
		t.Fatal("history aliases source")
	}
	live := []chat.Message{{Role: "system", Content: "current instructions"}, {Role: "user", Content: "它叫什么？"}}
	merged := withHistory(live, got)
	if len(merged) != 4 || merged[0].Content != "current instructions" || merged[1].Content != "项目代号松石" || merged[3].Content != "它叫什么？" {
		t.Fatal("incorrect prompt order")
	}
	if len(withHistory(live, nil)) != 2 {
		t.Fatal("empty session reused another history")
	}
}
func TestHistoryBoundsCompleteTurns(t *testing.T) {
	var history []chat.Message
	for i := 0; i < 25; i++ {
		history = append(history, chat.Message{Role: "user", Content: strings.Repeat("x", 2000)}, chat.Message{Role: "assistant", Content: strings.Repeat("y", 2000)})
	}
	got, err := conversationHistory(history)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 32 {
		t.Fatal("byte bound or paired trimming failed", len(got))
	}
	if _, err = conversationHistory([]chat.Message{{Role: "user", Content: strings.Repeat("x", 65537)}, {Role: "assistant", Content: "answer"}}); err == nil {
		t.Fatal("oversized newest turn silently forgotten")
	}
}

func TestHistoryPreservesHostTurnBoundaryAndCanonicalAnswer(t *testing.T) {
	input := []chat.Message{{Role: "user", Content: "审查甲公司合同", TurnID: "a"}, {Role: "assistant", Content: "intermediate", TurnID: "a"}, {Role: "user", Content: "重点看违约条款", TurnID: "a"}, {Role: "assistant", Content: "最终答案", TurnID: "a"}, {Role: "user", Content: "乙公司", TurnID: "b"}, {Role: "assistant", Content: "乙答案", TurnID: "b"}}
	got, err := conversationHistory(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0].Content != "审查甲公司合同\n\n重点看违约条款" || got[1].Content != "最终答案" {
		t.Fatal("host turn split or user instruction lost", got)
	}
}

func TestNativeMemoryControlsAllAcceptedTypeSpellings(t *testing.T) {
	tweaks := map[string]any{}
	if err := disableNativeMemory([]byte(`{"nodes":[{"id":"a","data":{"type":"Agent@v1"}},{"id":"i","data":{"type":"ext:teknowra:ChatInput@v1"}},{"id":"o","data":{"type":"ChatOutput@v1"}}]}`), tweaks); err != nil {
		t.Fatal(err)
	}
	if tweaks["a"] == nil || tweaks["i"] == nil || tweaks["o"] == nil {
		t.Fatal("accepted native type escaped memory controls")
	}
}
