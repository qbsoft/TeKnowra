package langflowrelease

import (
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"strings"
	"testing"
)

func reviewedGraphFixture() (map[string]any, langflowdef.Binding, map[string]string) {
	approved := map[string]string{}
	nodes := []any{}
	add := func(id, kind string, values map[string]any) {
		approved[kind] = kind
		fields := map[string]any{"code": map[string]any{"value": kind}}
		for k, v := range values {
			fields[k] = map[string]any{"value": v}
		}
		nodes = append(nodes, map[string]any{"id": id, "data": map[string]any{"type": kind, "node": map[string]any{"template": fields}}})
	}
	add("input", "ChatInput", nil)
	add("model", "TeKnowraModel", map[string]any{"model_id": "m"})
	add("knowledge", "TeKnowraKnowledge", map[string]any{"knowledge_base_id": "kb"})
	add("mcp", "TeKnowraMCP", map[string]any{"service_id": "s"})
	add("contract", "ReviewedExpert", map[string]any{"role": "contract", "original_request": ""})
	add("credit", "ReviewedExpert", map[string]any{"role": "credit", "original_request": ""})
	add("main", "Agent", nil)
	add("merge", "ExpertResults", map[string]any{"required_experts": "contract,credit"})
	add("output", "ChatOutput", nil)
	edges := []any{}
	edge := func(s, t, field string) {
		handle, _ := json.Marshal(map[string]string{"id": t, "fieldName": field})
		edges = append(edges, map[string]any{"source": s, "target": t, "targetHandle": string(handle)})
	}
	edge("model", "main", "model")
	for _, id := range []string{"contract", "credit"} {
		edge("input", id, "original_request")
		edge("model", id, "model")
		edge(id, "main", "tools")
		edge(id, "merge", "expert_tools")
	}
	edge("knowledge", "contract", "resource_tools")
	edge("mcp", "credit", "resource_tools")
	edge("main", "merge", "coordination")
	edge("merge", "output", "input_value")
	for _, e := range edges {
		entry := e.(map[string]any)
		h, _ := json.Marshal(map[string]string{"id": entry["source"].(string), "name": "report"})
		entry["sourceHandle"] = string(h)
	}
	b := langflowdef.Binding{ReviewedExperts: true, AgentID: "a", FlowID: "f", WorkspaceID: 1, Models: []string{"m"}, KnowledgeBases: []string{"kb"}, Tools: []langflowdef.ToolBinding{{ServiceID: "s", Name: "find_customer"}}, Nodes: []langflowdef.NodeBinding{{ID: "model", Kind: "model"}, {ID: "knowledge", Kind: "knowledge"}, {ID: "mcp", Kind: "mcp"}}}
	return map[string]any{"nodes": nodes, "edges": edges}, b, approved
}

func TestReviewedGraphRejectsOutputBypasses(t *testing.T) {
	for _, variant := range []string{"agent-output", "second-output", "missing-output", "wrong-output-port", "second-agent", "output-id-prefix", "output-node-flag"} {
		t.Run(variant, func(t *testing.T) {
			graph, b, approved := reviewedGraphFixture()
			nodes := graph["nodes"].([]any)
			edges := graph["edges"].([]any)
			switch variant {
			case "output-node-flag":
				for _, n := range nodes {
					node := n.(map[string]any)
					if node["id"] == "main" {
						node["data"].(map[string]any)["node"].(map[string]any)["is_output"] = true
					}
				}
			case "output-id-prefix":
				for _, n := range nodes {
					node := n.(map[string]any)
					if node["id"] == "main" {
						node["id"] = "ChatOutput-coordinator"
					}
				}
				for _, e := range edges {
					entry := e.(map[string]any)
					for _, key := range []string{"source", "target"} {
						if entry[key] == "main" {
							entry[key] = "ChatOutput-coordinator"
						}
					}
					for _, key := range []string{"sourceHandle", "targetHandle"} {
						entry[key] = strings.ReplaceAll(entry[key].(string), `"main"`, `"ChatOutput-coordinator"`)
					}
				}
			case "agent-output":
				for _, e := range edges {
					if e.(map[string]any)["target"] == "output" {
						e.(map[string]any)["source"] = "main"
					}
				}
			case "second-output", "second-agent":
				kind := "ChatOutput"
				if variant == "second-agent" {
					kind = "Agent"
				}
				nodes = append(nodes, map[string]any{"id": "bypass", "data": map[string]any{"type": kind, "node": map[string]any{"template": map[string]any{"code": map[string]any{"value": kind}}}}})
			case "missing-output":
				nodes = nodes[:len(nodes)-1]
			case "wrong-output-port":
				for _, e := range edges {
					if e.(map[string]any)["target"] == "output" {
						e.(map[string]any)["sourceHandle"] = `{"id":"merge","name":"unreviewed"}`
					}
				}
			}
			graph["nodes"] = nodes
			raw, _ := json.Marshal(graph)
			if ValidateGraph(raw, b, approved) == nil {
				t.Fatal("reviewed output bypass accepted")
			}
		})
	}
}

func TestReviewedGraphRequiresPinnedHostModelsAndRoleResources(t *testing.T) {
	graph, b, approved := reviewedGraphFixture()
	raw, _ := json.Marshal(graph)
	if err := ValidateGraph(raw, b, approved); err != nil {
		t.Fatal("valid reviewed graph rejected", err)
	}
	b.ReviewedExperts = false
	if ValidateGraph(raw, b, approved) == nil {
		t.Fatal("new component accepted in legacy profile")
	}
	graph, b, approved = reviewedGraphFixture()
	for _, e := range graph["edges"].([]any) {
		edge := e.(map[string]any)
		if edge["target"] == "contract" && edge["source"] == "knowledge" {
			edge["source"] = "mcp"
		}
	}
	raw, _ = json.Marshal(graph)
	if ValidateGraph(raw, b, approved) == nil {
		t.Fatal("contract expert accepted MCP instead of knowledge")
	}
}

func TestReviewedGraphRejectsLiteralTasksAndMissingMergeExpert(t *testing.T) {
	graph, b, approved := reviewedGraphFixture()
	for _, n := range graph["nodes"].([]any) {
		node := n.(map[string]any)
		if node["id"] == "credit" {
			fields := node["data"].(map[string]any)["node"].(map[string]any)["template"].(map[string]any)
			fields["original_request"] = map[string]any{"value": "查询另一客户"}
		}
	}
	raw, _ := json.Marshal(graph)
	if ValidateGraph(raw, b, approved) == nil {
		t.Fatal("literal task accepted")
	}
	graph, b, approved = reviewedGraphFixture()
	edges := []any{}
	for _, e := range graph["edges"].([]any) {
		edge := e.(map[string]any)
		if edge["source"] != "credit" || edge["target"] != "merge" {
			edges = append(edges, e)
		}
	}
	graph["edges"] = edges
	raw, _ = json.Marshal(graph)
	if ValidateGraph(raw, b, approved) == nil {
		t.Fatal("incomplete result merge accepted")
	}
}
