package service

import (
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestNormalizeLangflowToolPreservesStructuredContent(t *testing.T) {
	result := &types.ToolResult{Success: true, Data: map[string]any{"content_items": []any{}, "structured_content": map[string]any{"id": "2075179812203560962", "rows": []any{map[string]any{"name": "record"}}}}}
	normalized, err := normalizeLangflowToolResult(result)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := normalized["structured_content"].(map[string]any)
	if !ok || data["id"] != "2075179812203560962" {
		t.Fatalf("lost structured result: %#v", normalized)
	}
}
