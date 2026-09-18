package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebrandPromptTemplate(t *testing.T) {
	in := "You are WeKnora, an assistant developed by Tencent.\n" +
		"You are the WeKnora Skill Installer.\n" +
		"write `.weknora/requirements.json` in the skill directory"
	got := string(rebrandPromptTemplate([]byte(in)))

	for _, banned := range []string{"WeKnora", "Tencent"} {
		if strings.Contains(got, banned) {
			t.Errorf("rebranded template still contains %q:\n%s", banned, got)
		}
	}
	if !strings.Contains(got, "You are TeKnowra, an assistant developed by TyerSoft.") {
		t.Errorf("persona line not rebranded as expected:\n%s", got)
	}
	// 小写路径是技能机制依赖的真实目录名，不是品牌
	if !strings.Contains(got, ".weknora/requirements.json") {
		t.Errorf("lowercase .weknora path must be left untouched:\n%s", got)
	}
}

// 钩子挂在上游文件 config.go 的 loadPromptTemplates 里。合并上游时那一行若被冲掉，
// 这条测试会红：用仓库里真实的模板目录走一遍加载，结果里不能再出现上游品牌。
func TestLoadPromptTemplatesAppliesRebrand(t *testing.T) {
	configDir := filepath.Join("..", "..", "config")
	if _, err := os.Stat(filepath.Join(configDir, "prompt_templates")); err != nil {
		t.Skipf("prompt_templates dir not found: %v", err)
	}
	cfg, err := loadPromptTemplates(configDir)
	if err != nil || cfg == nil {
		t.Fatalf("loadPromptTemplates: cfg=%v err=%v", cfg, err)
	}

	groups := map[string][]PromptTemplate{
		"system_prompt":       cfg.SystemPrompt,
		"agent_system_prompt": cfg.AgentSystemPrompt,
		"intent_prompts":      cfg.IntentPrompts,
		"fallback":            cfg.Fallback,
	}
	seenTeKnowra := false
	for group, templates := range groups {
		if len(templates) == 0 {
			t.Errorf("%s: no templates loaded", group)
		}
		for _, tpl := range templates {
			if strings.Contains(tpl.Content, "WeKnora") || strings.Contains(tpl.Content, "developed by Tencent") {
				t.Errorf("%s/%s still carries upstream branding", group, tpl.ID)
			}
			if strings.Contains(tpl.Content, "You are TeKnowra") {
				seenTeKnowra = true
			}
		}
	}
	if !seenTeKnowra {
		t.Error("no template introduces itself as TeKnowra — did upstream change the persona wording?")
	}
}
