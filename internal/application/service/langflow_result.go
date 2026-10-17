package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/types"
)

func decodeLangflowStructured(result *types.ToolResult) (any, error) {
	raw, err := json.Marshal(result.Data["structured_content"])
	if err != nil || len(raw) > 128*1024 {
		return nil, fmt.Errorf("结构化工具结果超限")
	}
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err = d.Decode(&value); err != nil {
		return nil, fmt.Errorf("结构化工具结果不可解析")
	}
	return value, nil
}
func decodeLangflowBusinessObject(result *types.ToolResult) (map[string]any, error) {
	if result.Data["structured_content"] != nil {
		v, err := decodeLangflowStructured(result)
		if err != nil {
			return nil, err
		}
		if object, ok := v.(map[string]any); ok {
			return object, nil
		}
		return nil, fmt.Errorf("业务结果必须为JSON对象")
	}
	return decodeEinoBusinessResult(result)
}
func normalizeLangflowToolResult(result *types.ToolResult) (map[string]any, error) {
	if result == nil || !result.Success {
		return nil, fmt.Errorf("工具执行未成功")
	}
	if len(result.Images) > 0 {
		return nil, fmt.Errorf("本阶段不支持工具图片输出")
	}
	raw, err := json.Marshal(result.Data["content_items"])
	if err != nil || len(raw) > 128*1024 {
		return nil, fmt.Errorf("工具结果超限")
	}
	var items []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err = json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("工具结果不可解析")
	}
	for _, item := range items {
		if item.Type != "text" {
			return nil, fmt.Errorf("本阶段仅支持文本与JSON工具结果")
		}
	}
	if result.Data["structured_content"] != nil {
		value, err := decodeLangflowStructured(result)
		if err != nil {
			return nil, err
		}
		return map[string]any{"structured_content": value, "content": items}, nil
	}
	if data, err := decodeEinoBusinessResult(result); err == nil {
		return data, nil
	}
	return map[string]any{"content": items}, nil
}
