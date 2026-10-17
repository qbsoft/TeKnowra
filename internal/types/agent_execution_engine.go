package types

import (
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/einodef"
)

func ValidateAgentExecutionEngine(engine string) error {
	switch engine {
	case "", "native", "langflow", "eino":
		return nil
	default:
		return fmt.Errorf("不支持的执行引擎")
	}
}
func (c CustomAgentConfig) ValidateExecutionEngine() error {
	if err := ValidateAgentExecutionEngine(c.ExecutionEngine); err != nil {
		return err
	}
	if (c.ExecutionEngine == "langflow" || c.ExecutionEngine == "eino") && c.AgentMode != AgentModeSmartReasoning {
		return fmt.Errorf("所选执行引擎需要智能推理模式")
	}
	if c.ExecutionEngine == "eino" {
		d, ok := einodef.Lookup(c.EinoDefinition)
		if !ok {
			return fmt.Errorf("请选择已发布的Eino场景定义")
		}
		if c.SkillsSelectionMode != "" && c.SkillsSelectionMode != "none" || c.SandboxConfigID != "" || c.AllowSessionAttachments || c.ImageUploadEnabled || c.WebSearchEnabled || c.MemoryEnabled == nil || *c.MemoryEnabled || c.HistoryTurns != 0 || c.MultiTurnEnabled {
			return fmt.Errorf("Eino当前仅支持单次只读文本任务，请关闭技能、沙箱、附件、图片、网络搜索、长期记忆，并将历史轮数设为0")
		}
		return einodef.ValidateSelection(d, einodef.Selection{Knowledge: c.KBSelectionMode == "selected" && len(c.KnowledgeBases) > 0, MCPMode: c.MCPSelectionMode, Services: c.MCPServices, Tools: c.AllowedTools})
	} else if c.EinoDefinition != "" {
		return fmt.Errorf("Eino场景定义只能用于Eino执行引擎")
	}
	if c.ExecutionEngine == "langflow" && (c.WebSearchEnabled || c.MemoryEnabled != nil && *c.MemoryEnabled || c.ImageUploadEnabled) {
		return fmt.Errorf("Langflow当前不支持网络搜索、长期记忆或图片理解，请关闭后保存")
	}
	return nil
}
