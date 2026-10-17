package langflowruntime

import (
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/models/chat"
)

func disableNativeMemory(raw json.RawMessage, tweaks map[string]any) error {
	var graph struct {
		Nodes []struct {
			ID   string `json:"id"`
			Data struct {
				Type string `json:"type"`
			} `json:"data"`
		} `json:"nodes"`
	}
	if json.Unmarshal(raw, &graph) != nil {
		return fmt.Errorf("发布图无法读取记忆控制节点")
	}
	for _, node := range graph.Nodes {
		switch langflowdef.ComponentKind(node.Data.Type) {
		case "Agent":
			tweaks[node.ID] = map[string]any{"n_messages": 0}
		case "ChatInput", "ChatOutput":
			tweaks[node.ID] = map[string]any{"should_store_message": false}
		}
	}
	return nil
}

const maxConversationBytes = 64 * 1024
const maxConversationTurns = 20

// History is provided by the authorized host session, never Langflow memory.
// Only complete textual user/final-answer pairs are conversation context;
// old tools, credentials, reasoning and system prompts are not replayed.
func conversationHistory(input []chat.Message) ([]chat.Message, error) {
	var turns [][2]chat.Message
	var grouped [][2]chat.Message
	byID := map[string]int{}
	var pending *chat.Message
	for _, m := range input {
		if m.Content == "" || len(m.ToolCalls) > 0 || m.ToolCallID != "" {
			continue
		}
		if m.Kind == chat.MessageKindCompactionSummary {
			continue
		}
		if m.TurnID != "" && (m.Role == "user" || m.Role == "assistant") {
			index, ok := byID[m.TurnID]
			if !ok {
				index = len(grouped)
				byID[m.TurnID] = index
				grouped = append(grouped, [2]chat.Message{})
			}
			if m.Role == "user" {
				if grouped[index][0].Content != "" {
					grouped[index][0].Content += "\n\n"
				}
				grouped[index][0].Role = "user"
				grouped[index][0].Content += m.Content
			} else {
				grouped[index][1] = chat.Message{Role: "assistant", Content: m.Content}
			}
			continue
		}
		switch m.Role {
		case "user":
			if m.Kind == chat.MessageKindCompactionSummary {
				continue
			}
			text := chat.Message{Role: "user", Content: m.Content}
			pending = &text
		case "assistant":
			if pending != nil {
				turns = append(turns, [2]chat.Message{*pending, {Role: "assistant", Content: m.Content}})
				pending = nil
			}
		}
	}
	for _, pair := range grouped {
		if pair[0].Content != "" && pair[1].Content != "" {
			turns = append(turns, pair)
		}
	}
	start, total := len(turns), 0
	for i := len(turns) - 1; i >= 0 && len(turns)-i <= maxConversationTurns; i-- {
		size := len(turns[i][0].Content) + len(turns[i][1].Content)
		if total+size > maxConversationBytes {
			if i == len(turns)-1 {
				return nil, fmt.Errorf("最近一轮文本历史超过64KiB，请缩短内容或开始新会话")
			}
			break
		}
		total += size
		start = i
	}
	result := make([]chat.Message, 0, 2*(len(turns)-start))
	for _, turn := range turns[start:] {
		result = append(result, turn[0], turn[1])
	}
	return result, nil
}

func withHistory(live, history []chat.Message) []chat.Message {
	first := 0
	for first < len(live) && (live[first].Role == "system" || live[first].Role == "developer") {
		first++
	}
	result := make([]chat.Message, 0, len(live)+len(history))
	result = append(result, live[:first]...)
	result = append(result, history...)
	return append(result, live[first:]...)
}
