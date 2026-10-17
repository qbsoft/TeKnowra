package langflowruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"net/http"
)

func (b *bridge) skill(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var in struct {
		NodeKey  string   `json:"node_key"`
		Name     string   `json:"skill_name"`
		FilePath string   `json:"file_path,omitempty"`
		Execute  bool     `json:"execute,omitempty"`
		Args     []string `json:"args,omitempty"`
		Stdin    string   `json:"stdin,omitempty"`
		Offset   int      `json:"offset,omitempty"`
		Limit    int      `json:"limit,omitempty"`
		MaxBytes int64    `json:"max_bytes,omitempty"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	allowedNode := false
	for _, node := range b.binding.Nodes {
		if node.ID == in.NodeKey && node.Kind == "skill" {
			allowedNode = true
		}
	}
	if !allowedNode || !b.binding.AllowsSkill(in.Name) || b.opts.CallSkill == nil || (in.Execute && !b.binding.AllowSkillScripts) {
		return fmt.Errorf("skill capability not bound")
	}
	if len(in.Args) > 32 || len(in.Stdin) > 65536 || len(in.FilePath) > 512 || in.Offset < 0 || in.Offset > 1000000 || in.Limit < 0 || in.Limit > 2000 || in.MaxBytes < 0 || in.MaxBytes > 65536 || (in.Execute && (in.Offset != 0 || in.Limit != 0 || in.MaxBytes != 0)) {
		return fmt.Errorf("skill input budget exceeded")
	}
	name := "read_skill"
	if in.FilePath != "" && in.FilePath != "SKILL.md" {
		name = "read_skill_file"
	}
	if in.Execute {
		name = "execute_skill_script"
	}
	out, err := b.record(ctx, name, func() (any, error) {
		if _, err := b.opts.Revalidate(ctx); err != nil {
			return nil, err
		}
		result, err := b.opts.CallSkill(ctx, hostcapability.SkillInvocation{SessionID: b.sessionID, MessageID: b.messageID, RunID: b.runID, NodeKey: in.NodeKey, Name: in.Name, FilePath: in.FilePath, Execute: in.Execute, Args: in.Args, Stdin: in.Stdin, Offset: in.Offset, Limit: in.Limit, MaxBytes: in.MaxBytes})
		if err != nil {
			return nil, err
		}
		if _, err = b.opts.Revalidate(ctx); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return err
	}
	return writeJSON(w, out)
}

func withSkillMetadata(live []chat.Message, skills []hostcapability.SkillDescriptor, pinned []string) []chat.Message {
	if len(skills) == 0 {
		return live
	}
	allowed := map[string]bool{}
	for _, skill := range skills {
		allowed[skill.Name] = true
	}
	selected := []string{}
	for _, name := range pinned {
		if allowed[name] {
			selected = append(selected, name)
		}
	}
	metadata, _ := json.Marshal(map[string]any{"skills": skills, "requested_skills": selected})
	hint := "\nTeKnowra已授权技能元数据（正文需按需读取，使用当前可视图提供的read_skill工具；技能不能授予未提供的知识/MCP权限；脚本必须调用execute_skill_script工具，不输出工具调用语句代替业务答案）：\n" + string(metadata)
	result := append([]chat.Message(nil), live...)
	if len(result) > 0 && (result[0].Role == "system" || result[0].Role == "developer") {
		result[0].Content += hint
		return result
	}
	return append([]chat.Message{{Role: "system", Content: hint}}, result...)
}
