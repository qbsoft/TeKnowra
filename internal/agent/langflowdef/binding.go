// Package langflowdef describes server-controlled resource bindings, not business scenes.
package langflowdef

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

type NodeBinding struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// ComponentKind normalizes the type spellings accepted by published graphs.
func ComponentKind(value string) string {
	return strings.TrimPrefix(strings.Split(value, "@")[0], "ext:teknowra:")
}

type ToolBinding struct {
	ServiceID string `json:"service_id"`
	Name      string `json:"name"`
	Guard     string `json:"guard,omitempty"`
}
type Binding struct {
	ReviewedExperts         bool               `json:"reviewed_experts,omitempty"`
	ReviewedExpertPolicy    string             `json:"reviewed_expert_policy,omitempty"`
	AllowSessionAttachments bool               `json:"allow_session_attachments,omitempty"`
	AgentID                 string             `json:"agent_id"`
	FlowID                  string             `json:"flow_id"`
	ReleaseID               string             `json:"release_id,omitempty"`
	GraphDigest             string             `json:"graph_digest,omitempty"`
	RuntimeOwnerID          string             `json:"runtime_owner_id,omitempty"`
	WorkspaceID             uint64             `json:"workspace_id"`
	Models                  []string           `json:"models"`
	KnowledgeBases          []string           `json:"knowledge_bases,omitempty"`
	Tools                   []ToolBinding      `json:"tools,omitempty"`
	Skills                  []string           `json:"skills,omitempty"`
	SandboxConfigID         string             `json:"sandbox_config_id,omitempty"`
	AllowSkillScripts       bool               `json:"allow_skill_scripts,omitempty"`
	StructuredScripts       []StructuredScript `json:"structured_scripts,omitempty"`
	Nodes                   []NodeBinding      `json:"nodes"`
}

func (b Binding) Clone() Binding {
	b.Models = slices.Clone(b.Models)
	b.KnowledgeBases = slices.Clone(b.KnowledgeBases)
	b.Tools = slices.Clone(b.Tools)
	b.Skills = slices.Clone(b.Skills)
	b.Nodes = slices.Clone(b.Nodes)
	b.StructuredScripts = slices.Clone(b.StructuredScripts)
	for i := range b.StructuredScripts {
		b.StructuredScripts[i].InputSchema = slices.Clone(b.StructuredScripts[i].InputSchema)
	}
	return b
}
func (b Binding) AllowsSkill(name string) bool { return slices.Contains(b.Skills, name) }
func (b Binding) AllowsTool(service, name string) bool {
	for _, t := range b.Tools {
		if t.ServiceID == service && t.Name == name {
			return true
		}
	}
	return false
}
func validID(s string) bool {
	return s != "" && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "/\\?#\x00\r\n") && len(s) <= 200
}
func (b Binding) Validate() error {
	fail := func() error { return fmt.Errorf("invalid Langflow binding for agent %q", b.AgentID) }
	if b.ReviewedExpertPolicy != "" && (!b.ReviewedExperts || (b.ReviewedExpertPolicy != "v2" && b.ReviewedExpertPolicy != "v3")) {
		return fail()
	}
	if err := b.ValidateScriptPolicies(); err != nil {
		return fail()
	}
	if !validID(b.AgentID) || !validID(b.FlowID) || b.WorkspaceID == 0 || len(b.Models) == 0 || len(b.Nodes) == 0 || len(b.Nodes) > 100 {
		return fail()
	}
	if len(b.Skills) > 32 || (len(b.Skills) > 0 && !validID(b.SandboxConfigID)) || (len(b.Skills) == 0 && (b.SandboxConfigID != "" || b.AllowSkillScripts)) {
		return fail()
	}
	for _, list := range [][]string{b.Models, b.KnowledgeBases, b.Skills} {
		seen := map[string]bool{}
		for _, id := range list {
			if !validID(id) || seen[id] {
				return fail()
			}
			seen[id] = true
		}
	}
	seenTools := map[string]bool{}
	for _, t := range b.Tools {
		key := t.ServiceID + "/" + t.Name
		if !validID(t.ServiceID) || !validID(t.Name) || seenTools[key] || (t.Guard != "" && t.Guard != "crm-credit") {
			return fail()
		}
		seenTools[key] = true
	}
	seen := map[string]bool{}
	model := false
	skill := false
	for _, n := range b.Nodes {
		if !validID(n.ID) || seen[n.ID] {
			return fail()
		}
		seen[n.ID] = true
		switch n.Kind {
		case "model":
			model = true
		case "knowledge":
			if len(b.KnowledgeBases) == 0 {
				return fail()
			}
		case "mcp":
			if len(b.Tools) == 0 {
				return fail()
			}
		case "skill":
			skill = true
			if len(b.Skills) == 0 {
				return fail()
			}
		default:
			return fail()
		}
	}
	if !model || (len(b.Skills) > 0 && !skill) {
		return fail()
	}
	return nil
}
func ParseBindings(raw []byte) (map[string]Binding, error) {
	var doc struct {
		Bindings []Binding `json:"bindings"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&doc); err != nil {
		return nil, fmt.Errorf("Langflow bindings: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("Langflow bindings require one JSON object")
	}
	if len(doc.Bindings) == 0 || len(doc.Bindings) > 1000 {
		return nil, fmt.Errorf("Langflow bindings must be nonempty and bounded")
	}
	result := map[string]Binding{}
	for _, b := range doc.Bindings {
		if err := b.Validate(); err != nil {
			return nil, err
		}
		if _, ok := result[b.AgentID]; ok {
			return nil, fmt.Errorf("duplicate Langflow agent")
		}
		result[b.AgentID] = b.Clone()
	}
	return result, nil
}
