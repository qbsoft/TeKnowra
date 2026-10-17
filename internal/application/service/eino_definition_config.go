package service

import (
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/types"
)

// Validate host capabilities, never infer permissions from a business name.
func validateEinoDefinitionConfig(d einodef.Definition, c *types.AgentConfig) error {
	if c == nil || c.SharedAgentReadOnly || c.SkillsEnabled || c.SandboxConfigID != "" || c.WebSearchEnabled || c.LocalBrowserEnabled || c.MemoryEnabled == nil || *c.MemoryEnabled {
		return fmt.Errorf("%s需要本工作区只读能力，关闭技能、沙箱、网页和长期记忆", d.Name)
	}
	return einodef.ValidateSelection(d, einodef.Selection{Knowledge: len(c.SearchTargets) > 0 || (d.Knowledge && len(d.ReadOnlyTools) == 0), MCPMode: c.MCPSelectionMode, Services: c.MCPServices, Tools: c.AllowedTools})
}

// Compatibility wrappers for the existing pilot tests. Dispatch does not use them.
func validateEinoPilotConfig(c *types.AgentConfig) error {
	d, _ := einodef.Lookup(einodef.ContractReview)
	return validateEinoDefinitionConfig(d, c)
}
func validateEinoCreditConfig(c *types.AgentConfig) error {
	d, _ := einodef.Lookup(einodef.CustomerCredit)
	return validateEinoDefinitionConfig(d, c)
}
