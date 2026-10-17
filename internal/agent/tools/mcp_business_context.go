package tools

import "encoding/json"

// Bind only the business inquiry contract that advertises thread_id. The
// host's session is authoritative; model-generated IDs must not replace it.
func (t *MCPTool) bindBusinessSession(input MCPInput, meta *ToolExecContext) MCPInput {
	if t.mcpTool == nil || (t.mcpTool.Name != "ask_business" && t.mcpTool.Name != "analyze_sales_mechanism") || meta == nil || meta.SessionID == "" || input == nil {
		return input
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if json.Unmarshal(t.mcpTool.InputSchema, &schema) != nil {
		return input
	}
	if _, supported := schema.Properties["thread_id"]; !supported {
		return input
	}
	bound := make(MCPInput, len(input)+1)
	for key, value := range input {
		bound[key] = value
	}
	bound["thread_id"] = meta.SessionID
	return bound
}
