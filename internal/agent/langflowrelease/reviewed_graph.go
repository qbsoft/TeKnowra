package langflowrelease

import (
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"strings"
)

func validateReviewedGraph(raw json.RawMessage) error {
	fail := func() error {
		return fmt.Errorf("受控专家必须连接原始任务、宿主模型、职责资源及确定性结果汇总")
	}
	var graph struct {
		Nodes []struct {
			ID   string `json:"id"`
			Data struct {
				Type string `json:"type"`
				Node struct {
					Template map[string]json.RawMessage `json:"template"`
					IsOutput bool                       `json:"is_output"`
				} `json:"node"`
			} `json:"data"`
		} `json:"nodes"`
		Edges []struct {
			Source       string `json:"source"`
			Target       string `json:"target"`
			TargetHandle string `json:"targetHandle"`
			SourceHandle string `json:"sourceHandle"`
		} `json:"edges"`
	}
	if json.Unmarshal(raw, &graph) != nil {
		return fail()
	}
	kinds := map[string]string{}
	roles := map[string]string{}
	required := map[string][]string{}
	counts := map[string]int{}
	output := ""
	for _, node := range graph.Nodes {
		kind := langflowdef.ComponentKind(node.Data.Type)
		prefix := strings.SplitN(node.ID, "-", 2)[0]
		if kind != "ChatOutput" && (node.Data.Node.IsOutput || prefix == "ChatOutput" || prefix == "TextOutput" || prefix == "DataOutput") {
			return fail()
		}
		kinds[node.ID] = kind
		counts[kind]++
		value := func(key string) string {
			var f struct {
				Value string `json:"value"`
			}
			_ = json.Unmarshal(node.Data.Node.Template[key], &f)
			return f.Value
		}
		if kind == "ReviewedExpert" {
			role := value("role")
			if (role != "contract" && role != "credit") || value("original_request") != "" {
				return fail()
			}
			roles[node.ID] = role
		}
		if kind == "ExpertResults" {
			v := value("required_experts")
			if v != "contract,credit" && v != "contract" && v != "credit" {
				return fail()
			}
			required[node.ID] = strings.Split(v, ",")
		}
		if kind == "ChatOutput" {
			output = node.ID
			if value("input_value") != "" {
				return fail()
			}
		}
	}
	if len(roles) == 0 || len(required) != 1 || counts["ChatOutput"] != 1 || counts["Agent"] != 1 || counts["ChatInput"] != 1 {
		return fail()
	}
	inputs := map[string]map[string][]string{}
	outgoing := map[string]int{}
	for _, edge := range graph.Edges {
		outgoing[edge.Source]++
		if kinds[edge.Target] != "ReviewedExpert" && kinds[edge.Target] != "ExpertResults" && kinds[edge.Target] != "ChatOutput" {
			continue
		}
		var h struct {
			ID    string `json:"id"`
			Field string `json:"fieldName"`
		}
		if json.Unmarshal([]byte(strings.ReplaceAll(edge.TargetHandle, "œ", "\"")), &h) != nil || h.ID != edge.Target {
			return fail()
		}
		if inputs[edge.Target] == nil {
			inputs[edge.Target] = map[string][]string{}
		}
		inputs[edge.Target][h.Field] = append(inputs[edge.Target][h.Field], edge.Source)
		if edge.Target == output {
			var source struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if json.Unmarshal([]byte(strings.ReplaceAll(edge.SourceHandle, "œ", "\"")), &source) != nil || source.ID != edge.Source || source.Name != "report" || h.Field != "input_value" {
				return fail()
			}
		}
	}
	one := func(id, field, kind string) bool {
		list := inputs[id][field]
		return len(list) == 1 && kinds[list[0]] == kind
	}
	if !one(output, "input_value", "ExpertResults") || outgoing[output] != 0 {
		return fail()
	}
	// A second unconnected leaf may also be returned by Langflow as a message.
	for id := range kinds {
		if id != output && outgoing[id] == 0 {
			return fail()
		}
	}
	original := ""
	for id, role := range roles {
		resource := "TeKnowraKnowledge"
		if role == "credit" {
			resource = "TeKnowraMCP"
		}
		if !one(id, "model", "TeKnowraModel") || !one(id, "original_request", "ChatInput") || !one(id, "resource_tools", resource) {
			return fail()
		}
		task := inputs[id]["original_request"][0]
		if original != "" && task != original {
			return fail()
		}
		original = task
	}
	for id, wanted := range required {
		if !one(id, "coordination", "Agent") {
			return fail()
		}
		seen := map[string]bool{}
		if len(inputs[id]["expert_tools"]) != len(roles) {
			return fail()
		}
		for _, source := range inputs[id]["expert_tools"] {
			role := roles[source]
			if role == "" || seen[role] {
				return fail()
			}
			seen[role] = true
		}
		for _, role := range wanted {
			if !seen[role] {
				return fail()
			}
		}
		coordinator := inputs[id]["coordination"][0]
		// All merged experts must actually be available to this coordinator.
		for _, source := range inputs[id]["expert_tools"] {
			connected := false
			for _, edge := range graph.Edges {
				if edge.Source == source && edge.Target == coordinator {
					var h struct {
						Field string `json:"fieldName"`
					}
					if json.Unmarshal([]byte(strings.ReplaceAll(edge.TargetHandle, "œ", "\"")), &h) == nil && h.Field == "tools" {
						connected = true
					}
				}
			}
			if !connected {
				return fail()
			}
		}
	}
	return nil
}
