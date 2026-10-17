package langflowrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type VersionClient interface {
	Version(context.Context) (string, error)
}

// Only skill-enabled releases pin this additional component. Old profiles
// continue to verify against their original sources and package versions.
func WithSkillComponent(base Components, root string) (Components, error) {
	raw, err := os.ReadFile(filepath.Join(root, "components/teknowra/skills.py"))
	if err != nil || len(raw) > 1024*1024 {
		return Components{}, fmt.Errorf("技能运行组件源不可用")
	}
	source := strings.ReplaceAll(string(raw), "\r\n", "\n")
	approved := map[string]string{}
	for name, code := range base.Approved {
		approved[name] = code
	}
	approved["TeKnowraSkills"] = source
	payload, _ := json.Marshal(map[string]any{"base": base.Digest, "skill_component": source})
	digest, err := Digest(payload)
	return Components{Digest: digest, Approved: approved}, err
}

// LocalComponents pins the current installed sources; remote attestation belongs to deployment work.
func LocalComponents(ctx context.Context, client VersionClient, root, lib string) (Components, error) {
	version, err := client.Version(ctx)
	if err != nil {
		return Components{}, err
	}
	paths := map[string]string{"TeKnowraModel": filepath.Join(root, "components/teknowra/model.py"), "TeKnowraKnowledge": filepath.Join(root, "components/teknowra/knowledge.py"), "TeKnowraMCP": filepath.Join(root, "components/teknowra/mcp.py"), "support": filepath.Join(root, "teknowra_support.py"), "Agent": filepath.Join(lib, "lfx/components/models_and_agents/agent.py"), "ChatInput": filepath.Join(lib, "lfx/components/input_output/chat.py"), "ChatOutput": filepath.Join(lib, "lfx/components/input_output/chat_output.py")}
	sources := map[string]string{}
	for name, path := range paths {
		raw, e := os.ReadFile(path)
		if e != nil || len(raw) > 1024*1024 {
			return Components{}, fmt.Errorf("本机运行组件源不可用")
		}
		sources[name] = strings.ReplaceAll(string(raw), "\r\n", "\n")
	}
	packages := map[string]string{}
	for _, name := range []string{"lfx", "langflow_base", "langchain_core", "langchain_openai", "langchain", "langgraph", "langgraph_checkpoint", "langgraph_prebuilt", "langgraph_sdk", "pydantic", "jsonschema", "httpx"} {
		matches, e := filepath.Glob(filepath.Join(lib, name+"-*.dist-info/METADATA"))
		if e != nil || len(matches) != 1 {
			return Components{}, fmt.Errorf("本机运行依赖版本不可用")
		}
		raw, e := os.ReadFile(matches[0])
		if e != nil {
			return Components{}, fmt.Errorf("本机运行依赖版本不可用")
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "Version: ") {
				packages[name] = strings.TrimSpace(strings.TrimPrefix(line, "Version: "))
				break
			}
		}
		if packages[name] == "" {
			return Components{}, fmt.Errorf("本机运行依赖版本不可用")
		}
	}
	raw, err := json.Marshal(map[string]any{"version": version, "sources": sources, "packages": packages})
	if err != nil {
		return Components{}, err
	}
	digest, err := Digest(raw)
	return Components{Digest: digest, Approved: sources}, err
}
