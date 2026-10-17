package langflowrelease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
)

func GraphUsesReviewedExperts(raw json.RawMessage) bool {
	var graph struct {
		Nodes []struct {
			Data struct {
				Type string `json:"type"`
			} `json:"data"`
		} `json:"nodes"`
	}
	if json.Unmarshal(raw, &graph) != nil {
		return false
	}
	for _, n := range graph.Nodes {
		kind := langflowdef.ComponentKind(n.Data.Type)
		if kind == "ReviewedExpert" || kind == "ExpertResults" {
			return true
		}
	}
	return false
}

// Opt-in source pinning: existing fixed releases retain their original profile.
func WithReviewedExpertComponents(base Components, root string) (Components, error) {
	return WithReviewedExpertPolicy(base, root, "")
}

// A new policy gets separate immutable support sources. Older releases keep
// their original digest and module; publishing a fix never rewrites them.
func WithReviewedExpertPolicy(base Components, root, policy string) (Components, error) {
	if policy != "" && policy != "v2" && policy != "v3" {
		return Components{}, fmt.Errorf("未知专家复核版本")
	}
	approved := map[string]string{}
	for key, value := range base.Approved {
		approved[key] = value
	}
	paths := map[string]string{"ReviewedExpert": "components/teknowra/reviewed_expert.py", "ExpertResults": "components/teknowra/expert_results.py", "ReviewedExpertSupport": "teknowra_reviewed_experts.py"}
	if policy == "v2" || policy == "v3" {
		paths["ReviewedExpert"] = "components/teknowra/reviewed_expert_" + policy + ".py"
		paths["ReviewedExpertSupport"] = "teknowra_reviewed_experts_" + policy + ".py"
	}
	extra := map[string]string{}
	for name, path := range paths {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || len(raw) == 0 || len(raw) > 1024*1024 {
			return Components{}, fmt.Errorf("受控专家组件源不可用")
		}
		code := strings.ReplaceAll(string(raw), "\r\n", "\n")
		approved[name] = code
		extra[name] = code
	}
	raw, _ := json.Marshal(map[string]any{"base": base.Digest, "reviewed_expert_sources": extra})
	digest, err := Digest(raw)
	return Components{Digest: digest, Approved: approved}, err
}

func GraphReviewedExpertPolicy(raw json.RawMessage) string {
	var g struct {
		Nodes []struct {
			Data struct {
				Node struct {
					Template map[string]json.RawMessage `json:"template"`
				} `json:"node"`
			} `json:"data"`
		} `json:"nodes"`
	}
	if json.Unmarshal(raw, &g) != nil {
		return ""
	}
	for _, n := range g.Nodes {
		var field struct {
			Value string `json:"value"`
		}
		if json.Unmarshal(n.Data.Node.Template["code"], &field) != nil {
			continue
		}
		code := field.Value
		if strings.Contains(code, "from teknowra_reviewed_experts_v3 import") {
			return "v3"
		}
		if strings.Contains(code, "from teknowra_reviewed_experts_v2 import") {
			return "v2"
		}
	}
	return ""
}
