package mcp

import (
	"encoding/json"
	sdk "github.com/mark3labs/mcp-go/mcp"
	"testing"
)

func TestStructuredOnlyToolResultIsNotLost(t *testing.T) {
	result := convertCallToolResult(&sdk.CallToolResult{StructuredContent: map[string]any{"customerId": "2075179812203560962", "total": 500000}})
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	structured, ok := decoded["structuredContent"].(map[string]any)
	if !ok || structured["customerId"] != "2075179812203560962" {
		t.Fatalf("structured result lost: %s", raw)
	}
}
