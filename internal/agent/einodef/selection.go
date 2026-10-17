package einodef

import (
	"fmt"
	"strings"
)

// Selection is independent of storage/runtime structs, so both save and execute
// validate the same manifest. Authorization remains in the existing host checks.
type Selection struct {
	Knowledge       bool
	MCPMode         string
	Services, Tools []string
}

func ValidateSelection(d Definition, s Selection) error {
	if d.Knowledge && !s.Knowledge {
		return fmt.Errorf("%s需要指定知识库", d.Name)
	}
	expected := len(d.ReadOnlyTools)
	if d.Knowledge {
		expected++
	}
	if len(s.Tools) != expected {
		return fmt.Errorf("%s的工具组合与场景目录不符", d.Name)
	}
	if len(d.ReadOnlyTools) == 0 {
		if s.MCPMode != "none" || len(s.Services) != 0 {
			return fmt.Errorf("%s不支持MCP，请关闭MCP", d.Name)
		}
	} else if s.MCPMode != "selected" || len(s.Services) != 1 || s.Services[0] == "" {
		return fmt.Errorf("%s需要选择一个按人OAuth MCP服务", d.Name)
	}
	seen := map[string]bool{}
	for _, key := range s.Tools {
		if d.Knowledge && (key == "knowledge_search" || key == "search_knowledge") && !seen["knowledge"] {
			seen["knowledge"] = true
			continue
		}
		parts := strings.SplitN(key, ":", 3)
		if len(parts) != 3 || parts[0] != "mcp" || len(s.Services) != 1 || parts[1] != s.Services[0] || !d.AllowsTool(parts[2]) || seen[parts[2]] {
			return fmt.Errorf("工具不在已发布能力中")
		}
		seen[parts[2]] = true
	}
	return nil
}
