package langflowdef

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// StructuredScript constrains calls to operator-reviewed package code. It is
// NOT filesystem isolation for arbitrary scripts. The host checks package hashes.
type StructuredScript struct {
	Skill       string          `json:"skill"`
	Path        string          `json:"path"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func (p StructuredScript) schema() (*jsonschema.Schema, error) {
	if len(p.InputSchema) == 0 || len(p.InputSchema) > 16384 {
		return nil, fmt.Errorf("脚本输入schema缺失或过大")
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(p.InputSchema))
	if err != nil {
		return nil, err
	}
	root, ok := doc.(map[string]any)
	if !ok || root["type"] != "object" || root["additionalProperties"] != false {
		return nil, fmt.Errorf("脚本输入必须是禁止额外字段的JSON对象")
	}
	fields, ok := root["properties"].(map[string]any)
	if !ok || len(fields) == 0 || len(fields) > 32 {
		return nil, fmt.Errorf("脚本输入字段必须有界")
	}
	allowed := map[string]bool{"type": true, "properties": true, "required": true, "additionalProperties": true, "description": true}
	for key := range root {
		if !allowed[key] {
			return nil, fmt.Errorf("不支持的脚本schema关键字")
		}
	}
	for name, value := range fields {
		if !validID(name) {
			return nil, fmt.Errorf("无效输入字段")
		}
		field, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("无效输入字段schema")
		}
		switch field["type"] {
		case "string", "number", "integer", "boolean":
		default:
			return nil, fmt.Errorf("仅支持简单结构化字段")
		}
		for key := range field {
			switch key {
			case "type", "description", "enum", "minimum", "maximum", "minLength", "maxLength", "pattern":
			default:
				return nil, fmt.Errorf("不支持的输入字段约束")
			}
		}
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(nil)
	const url = "urn:teknowra:skill:structured-input"
	if err = c.AddResource(url, doc); err != nil {
		return nil, err
	}
	return c.Compile(url)
}

func (b Binding) ValidateScriptPolicies() error {
	if len(b.StructuredScripts) > 32 || (len(b.StructuredScripts) > 0 && !b.AllowSkillScripts) || (b.AllowSessionAttachments && b.AllowSkillScripts && len(b.StructuredScripts) == 0) {
		return fmt.Errorf("附件脚本需要已审核的固定入口与结构化输入策略")
	}
	seen := map[string]bool{}
	for _, p := range b.StructuredScripts {
		key := p.Skill + "/" + p.Path
		if !b.AllowsSkill(p.Skill) || !strings.HasPrefix(p.Path, "scripts/") || path.Clean(p.Path) != p.Path || strings.ContainsAny(p.Path, "\\;|&`$\"'\r\n\x00") || seen[key] {
			return fmt.Errorf("脚本入口不在授权技能内或重复")
		}
		switch path.Ext(p.Path) {
		case ".py", ".js", ".sh":
		default:
			return fmt.Errorf("不支持的技能脚本类型")
		}
		if _, err := p.schema(); err != nil {
			return fmt.Errorf("技能脚本输入策略无效: %w", err)
		}
		seen[key] = true
	}
	return nil
}

// ValidateScriptInput runs before sandbox initialization. Legacy non-attachment
// publications retain their existing script contract.
func (b Binding) ValidateScriptInput(skill, entry string, args []string, stdin string) error {
	if !b.AllowSkillScripts || !b.AllowsSkill(skill) {
		return fmt.Errorf("技能脚本未授权")
	}
	if !b.AllowSessionAttachments && len(b.StructuredScripts) == 0 {
		return nil
	}
	if len(args) != 0 || len(stdin) == 0 || len(stdin) > 65536 {
		return fmt.Errorf("受控计算仅接受有界JSON stdin，不接受命令行参数")
	}
	for _, p := range b.StructuredScripts {
		if p.Skill != skill || p.Path != entry {
			continue
		}
		schema, err := p.schema()
		if err != nil {
			return fmt.Errorf("技能脚本输入策略无效")
		}
		value, err := jsonschema.UnmarshalJSON(strings.NewReader(stdin))
		if err != nil {
			return fmt.Errorf("技能计算输入必须是单个JSON对象")
		}
		if err = schema.Validate(value); err != nil {
			return fmt.Errorf("技能计算输入不符合已发布字段与取值约束")
		}
		return nil
	}
	return fmt.Errorf("脚本入口未列入本发布的受控计算清单")
}
