package langflowrelease

import (
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"slices"
	"strings"
)

func resourceID(value any) string {
	text, _ := value.(string)
	parts := strings.Split(text, " · ")
	return parts[len(parts)-1]
}
func ValidateGraph(raw json.RawMessage, b langflowdef.Binding, approved map[string]string) error {
	fail := func() error {
		return fmt.Errorf("流程含未批准组件、动态依赖、临时凭据或未绑定资源，不能发布")
	}
	// Template includes a string _type field, so decode node metadata separately.
	var root struct {
		Nodes []struct {
			ID   string `json:"id"`
			Data struct {
				Type string `json:"type"`
				Node struct {
					Template map[string]json.RawMessage `json:"template"`
				} `json:"node"`
			} `json:"data"`
		} `json:"nodes"`
		Edges []struct {
			Source       string `json:"source"`
			Target       string `json:"target"`
			TargetHandle string `json:"targetHandle"`
		} `json:"edges"`
	}
	if json.Unmarshal(raw, &root) != nil || len(root.Nodes) == 0 || len(root.Nodes) > 100 {
		return fail()
	}
	kinds := map[string]string{}
	for _, node := range b.Nodes {
		kinds[node.ID] = node.Kind
	}
	found := map[string]bool{}
	nodeIDs := map[string]bool{}
	nodeKinds := map[string]string{}
	for _, node := range root.Nodes {
		if node.ID == "" || nodeIDs[node.ID] {
			return fail()
		}
		nodeIDs[node.ID] = true
		kind := langflowdef.ComponentKind(node.Data.Type)
		nodeKinds[node.ID] = kind
		allowed := slices.Contains([]string{"Agent", "ChatInput", "ChatOutput", "TeKnowraModel", "TeKnowraKnowledge", "TeKnowraMCP", "TeKnowraSkills"}, kind)
		if b.ReviewedExperts && (kind == "ReviewedExpert" || kind == "ExpertResults") {
			allowed = true
		}
		if !allowed {
			return fail()
		}
		fields := map[string]map[string]any{}
		for name, value := range node.Data.Node.Template {
			if name == "_type" {
				continue
			}
			if name == "is_refresh" {
				var flag bool
				if json.Unmarshal(value, &flag) != nil {
					return fail()
				}
				continue
			}
			var field map[string]any
			if json.Unmarshal(value, &field) != nil {
				return fail()
			}
			fields[name] = field
			if field["load_from_db"] == true || field["is_global"] == true {
				return fail()
			}
			if field["password"] == true && field["value"] != nil && field["value"] != "" {
				return fail()
			}
		}
		for _, name := range []string{"directory_connection", "task_capability", "bridge_url", "node_key", "tools_json"} {
			if fields[name] != nil && fields[name]["value"] != nil && fields[name]["value"] != "" {
				return fail()
			}
		}
		code, _ := fields["code"]["value"].(string)
		if code == "" || code != approved[kind] {
			return fail()
		}
		if kind == "ChatInput" {
			value := fields["files"]["value"]
			empty := value == nil || value == ""
			if list, ok := value.([]any); ok {
				empty = len(list) == 0
			}
			if !empty {
				return fail()
			}
		}
		if strings.HasPrefix(kind, "TeKnowra") {
			wanted := map[string]string{"TeKnowraModel": "model", "TeKnowraKnowledge": "knowledge", "TeKnowraMCP": "mcp", "TeKnowraSkills": "skill"}[kind]
			if kinds[node.ID] != wanted {
				return fail()
			}
			found[node.ID] = true
			switch wanted {
			case "skill":
				scripts, valid := fields["enable_scripts"]["value"].(bool)
				if !valid || !b.AllowsSkill(resourceID(fields["skill_name"]["value"])) || (scripts && !b.AllowSkillScripts) {
					return fail()
				}
			case "model":
				if !slices.Contains(b.Models, resourceID(fields["model_id"]["value"])) {
					return fail()
				}
			case "knowledge":
				if !slices.Contains(b.KnowledgeBases, resourceID(fields["knowledge_base_id"]["value"])) {
					return fail()
				}
			case "mcp":
				service := resourceID(fields["service_id"]["value"])
				selected, _ := fields["tool"]["value"].(string)
				present := false
				for _, tool := range b.Tools {
					if tool.ServiceID == service && (selected == "" || tool.Name == selected) {
						present = true
					}
				}
				if !present {
					return fail()
				}
			}
		} else if kinds[node.ID] != "" {
			return fail()
		}
	}
	if len(found) != len(b.Nodes) {
		return fail()
	}
	modelInputs := map[string]int{}
	for _, edge := range root.Edges {
		if !nodeIDs[edge.Source] || !nodeIDs[edge.Target] {
			return fail()
		}
		if nodeKinds[edge.Target] == "Agent" {
			var handle struct {
				FieldName string `json:"fieldName"`
				ID        string `json:"id"`
			}
			if json.Unmarshal([]byte(strings.ReplaceAll(edge.TargetHandle, "œ", "\"")), &handle) != nil || handle.ID != edge.Target {
				return fail()
			}
			if handle.FieldName == "model" {
				if nodeKinds[edge.Source] != "TeKnowraModel" {
					return fail()
				}
				modelInputs[edge.Target]++
			}
		}
	}
	// A connected host model overrides the native selector's saved UI default.
	// Disconnected/default provider selections must never resolve global credentials.
	for id, kind := range nodeKinds {
		if kind == "Agent" && modelInputs[id] != 1 {
			return fail()
		}
	}
	if b.ReviewedExperts {
		return validateReviewedGraph(raw)
	}
	return nil
}
