package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"
)

func validateLangflowSkillGrant(b langflowdef.Binding, c types.CustomAgentConfig) error {
	fail := func() error { return fmt.Errorf("技能或执行环境不在当前Agent授权范围") }
	if len(b.Skills) == 0 {
		return nil
	}
	if c.SandboxConfigID != b.SandboxConfigID {
		return fail()
	}
	switch c.SkillsSelectionMode {
	case "all":
		return nil
	case "selected":
		for _, name := range b.Skills {
			if !slices.Contains(c.SelectedSkills, name) {
				return fail()
			}
		}
		return nil
	default:
		return fail()
	}
}

func langflowSkillDigest(b langflowdef.Binding, rows []*types.TenantSkillEntity) (string, error) {
	type entry struct {
		Name, Version, Description, InstructionsHash, BundleHash, CatalogID string
		RequiresTools                                                       types.JSON
	}
	var manifest []entry
	for _, name := range b.Skills {
		found := false
		for _, row := range rows {
			if row == nil || row.Name != name || row.TenantID != b.WorkspaceID || row.SandboxConfigID != b.SandboxConfigID || !row.Enabled {
				continue
			}
			served := row.ServedView()
			if served == nil {
				continue
			}
			raw, err := hex.DecodeString(served.BundleSHA256)
			if err != nil || len(raw) != 32 {
				return "", fmt.Errorf("技能包没有可验证版本，请重新安装")
			}
			sum := sha256.Sum256([]byte(served.Instructions))
			manifest = append(manifest, entry{served.Name, served.Version, served.Description, hex.EncodeToString(sum[:]), served.BundleSHA256, served.CatalogID, served.RequiresTools})
			found = true
			break
		}
		if !found {
			return "", fmt.Errorf("绑定技能已禁用、未安装或不在当前执行镜像中")
		}
	}
	sort.Slice(manifest, func(i, j int) bool { return manifest[i].Name < manifest[j].Name })
	raw, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	return langflowrelease.Digest(raw)
}

type langflowSkillState struct {
	Rows        []*types.TenantSkillEntity
	Descriptors []hostcapability.SkillDescriptor
	Digest      string
}

// A session may retain its old sandbox after an Agent moves to another
// configuration. Never mix a new package/person's environment with that pin.
func validateLangflowSkillSession(ctx context.Context, pinner *SessionSandboxPinner, b langflowdef.Binding, sessionID string) error {
	if pinner == nil || sessionID == "" {
		return fmt.Errorf("技能会话执行环境无法核实")
	}
	pin, err := pinner.Read(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("技能会话执行环境无法核实")
	}
	if !pin.IsZero() && (pin.ConfigID != b.SandboxConfigID || pin.TenantOr(b.WorkspaceID) != b.WorkspaceID) {
		return fmt.Errorf("会话仍绑定旧技能执行环境，请新建会话")
	}
	return nil
}

func (s *agentService) langflowSkills(ctx context.Context, b langflowdef.Binding) (langflowSkillState, error) {
	var result langflowSkillState
	if len(b.Skills) == 0 {
		return result, nil
	}
	if s.db == nil {
		return result, fmt.Errorf("技能授权存储未准备")
	}
	var agent types.CustomAgent
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", b.AgentID, b.WorkspaceID).First(&agent).Error; err != nil {
		return result, fmt.Errorf("技能所属Agent不可用")
	}
	if err := validateLangflowSkillGrant(b, agent.Config); err != nil {
		return result, err
	}
	configs := repository.NewTenantSandboxConfigRepository(s.db)
	rows := effectiveTenantSkills(ctx, configs, repository.NewTenantSkillRepository(s.db), b.WorkspaceID, b.SandboxConfigID)
	for _, row := range rows {
		if row != nil && b.AllowsSkill(row.Name) {
			result.Rows = append(result.Rows, row)
		}
	}
	digest, err := langflowSkillDigest(b, result.Rows)
	if err != nil {
		return result, err
	}
	environment, err := configs.GetByID(ctx, b.WorkspaceID, b.SandboxConfigID)
	if err != nil || environment == nil || environment.Config == nil || environment.Config.SkillImage == nil {
		return result, fmt.Errorf("技能执行镜像不可用")
	}
	raw, _ := json.Marshal(map[string]any{"skills": digest, "snapshot_id": environment.Config.SkillImage.SnapshotID})
	result.Digest, err = langflowrelease.Digest(raw)
	if err != nil {
		return result, err
	}
	for _, row := range result.Rows {
		var required []string
		if len(row.RequiresTools) > 0 && json.Unmarshal(row.RequiresTools, &required) != nil {
			return result, fmt.Errorf("技能工具依赖格式无效")
		}
		result.Descriptors = append(result.Descriptors, hostcapability.SkillDescriptor{ID: row.Name, Name: row.Name, Description: row.Description, RequiresTools: required})
	}
	sort.Slice(result.Descriptors, func(i, j int) bool { return result.Descriptors[i].Name < result.Descriptors[j].Name })
	return result, nil
}

func shellLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func langflowSkillReadInput(in hostcapability.SkillInvocation) (tools.ReadFileInput, error) {
	file := in.FilePath
	if file == "" {
		file = "SKILL.md"
	}
	if path.IsAbs(file) || path.Clean(file) != file || strings.ContainsAny(file, "\\\r\n\x00") || strings.HasPrefix(file, "../") || in.Offset < 0 || in.Offset > 1000000 || in.Limit < 0 || in.Limit > 2000 || in.MaxBytes < 0 || in.MaxBytes > 65536 {
		return tools.ReadFileInput{}, fmt.Errorf("技能资源路径或分页参数无效")
	}
	offset, limit, maxBytes := in.Offset, in.Limit, in.MaxBytes
	if offset == 0 {
		offset = 1
	}
	if limit == 0 {
		limit = 2000
	}
	if maxBytes == 0 {
		maxBytes = 65536
	}
	return tools.ReadFileInput{Path: "skill://" + in.Name + "/" + file, Offset: offset, Limit: limit, MaxBytes: maxBytes}, nil
}
func skillScriptCommand(base, relative string, args []string) (string, error) {
	if relative == "" || path.IsAbs(relative) || path.Clean(relative) != relative || !strings.HasPrefix(relative, "scripts/") || strings.ContainsAny(relative, "\\;|&`$\"'\r\n\x00") || len(args) > 32 {
		return "", fmt.Errorf("只能运行绑定技能scripts目录中的相对脚本")
	}
	interpreter := ""
	switch path.Ext(relative) {
	case ".py":
		interpreter = "python"
	case ".js":
		interpreter = "node"
	case ".sh":
		interpreter = "bash"
	default:
		return "", fmt.Errorf("当前支持Python、Node和Bash技能脚本")
	}
	command := interpreter + " " + shellLiteral(path.Join(base, relative))
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return "", fmt.Errorf("技能参数含无效字符")
		}
		command += " " + shellLiteral(arg)
	}
	if len(command) > 8192 {
		return "", fmt.Errorf("技能脚本参数过长，请使用stdin传递正文")
	}
	return command, nil
}

// Reuses the platform registry and SkillManager. No skill body or environment
// credentials are copied into the Langflow graph or its metadata directory.
func (s *agentService) langflowSkillCaller(ctx context.Context, b langflowdef.Binding, c *types.AgentConfig, sessionID string, check func(context.Context) error) ([]hostcapability.SkillDescriptor, func(context.Context, hostcapability.SkillInvocation) (map[string]any, error), func(context.Context), error) {
	if err := validateLangflowSkillSession(ctx, s.sandboxPinner, b, sessionID); err != nil {
		return nil, nil, nil, err
	}
	initial, err := s.langflowSkills(ctx, b)
	if err != nil {
		return nil, nil, nil, err
	}
	local := *c
	local.SkillsEnabled = true
	local.SkillDirs = nil
	local.AllowedSkills = slices.Clone(b.Skills)
	local.TenantSkills = initial.Rows
	local.SandboxConfigID = b.SandboxConfigID
	registry := tools.NewToolRegistry()
	var once sync.Once
	var initErr error
	var skillDir func(string) (string, bool)
	recheck := func(current context.Context) error {
		if err := validateLangflowSkillSession(current, s.sandboxPinner, b, sessionID); err != nil {
			return err
		}
		if err := check(current); err != nil {
			return err
		}
		live, err := s.langflowSkills(current, b)
		if err != nil {
			return err
		}
		if live.Digest != initial.Digest {
			return fmt.Errorf("技能版本或执行镜像已变化，请重新发布")
		}
		return nil
	}
	call := func(current context.Context, in hostcapability.SkillInvocation) (map[string]any, error) {
		if !b.AllowsSkill(in.Name) || in.SessionID != sessionID || (in.Execute && !b.AllowSkillScripts) {
			return nil, fmt.Errorf("技能调用不在当前任务授权范围")
		}
		if err := recheck(current); err != nil {
			return nil, err
		}
		if in.Execute {
			if err := b.ValidateScriptInput(in.Name, in.FilePath, in.Args, in.Stdin); err != nil {
				return nil, err
			}
		}
		once.Do(func() {
			if b.AllowSkillScripts {
				s.registerSandboxShellIfAllowed(ctx, registry, sessionID, &local)
			}
			manager, err := s.initializeSkillsManager(ctx, sessionID, &local, registry)
			if err != nil {
				initErr = err
				return
			}
			skillDir = manager.SandboxSkillDir
			registry.BindSession(sessionID)
		})
		if initErr != nil {
			return nil, initErr
		}
		// Native initialization can adopt a concurrent first-sandbox winner.
		// Check again before reading resources or injecting per-person variables.
		if err := recheck(current); err != nil {
			return nil, err
		}
		name := tools.ToolReadFile
		var arguments []byte
		if !in.Execute {
			input, err := langflowSkillReadInput(in)
			if err != nil {
				return nil, err
			}
			arguments, _ = json.Marshal(input)
		} else {
			if _, err := registry.GetTool(tools.ToolShellExec); err != nil {
				return nil, fmt.Errorf("当前平台执行环境没有技能脚本能力")
			}
			if skillDir == nil {
				return nil, fmt.Errorf("技能执行目录不可用")
			}
			base, ok := skillDir(in.Name)
			if !ok {
				return nil, fmt.Errorf("技能不在已安装执行镜像中")
			}
			command, err := skillScriptCommand(base, in.FilePath, in.Args)
			if err != nil {
				return nil, err
			}
			// Verify the script exists in the immutable installed package before shell execution.
			reader, _ := json.Marshal(tools.ReadFileInput{Path: "skill://" + in.Name + "/" + in.FilePath, MaxBytes: 1024})
			result, err := registry.ExecuteTool(current, tools.ToolReadFile, reader)
			if err != nil || result == nil || !result.Success {
				return nil, fmt.Errorf("技能脚本资源不可用")
			}
			name = tools.ToolShellExec
			arguments, _ = json.Marshal(tools.ShellExecInput{Command: command, SkillName: in.Name, Stdin: in.Stdin, TimeoutSec: 120, MaxOutputBytes: 16384})
		}
		result, err := registry.ExecuteTool(current, name, arguments)
		if err != nil || result == nil || !result.Success {
			return nil, fmt.Errorf("受控技能调用失败，请检查技能资源、运行环境或个人变量")
		}
		if err := recheck(current); err != nil {
			return nil, err
		}
		aliases := langflowSkillToolAliases(b)
		var missing []string
		for _, meta := range initial.Descriptors {
			if meta.Name == in.Name {
				for _, required := range meta.RequiresTools {
					if len(aliases[required]) == 0 {
						missing = append(missing, required)
					}
				}
			}
		}
		return map[string]any{"output": result.Output, "data": result.Data, "skill_name": in.Name, "tool_aliases": aliases, "unavailable_required_tools": missing}, nil
	}
	return initial.Descriptors, call, func(current context.Context) { registry.Cleanup(current) }, nil
}

func langflowToolAlias(source, name string) string {
	sum := sha256.Sum256([]byte(source + "/" + name))
	var suffix strings.Builder
	for _, r := range name {
		if suffix.Len() == 40 {
			break
		}
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			suffix.WriteRune(r)
		} else {
			suffix.WriteByte('_')
		}
	}
	return "tk_" + hex.EncodeToString(sum[:])[:12] + "_" + suffix.String()
}
func langflowSkillToolAliases(b langflowdef.Binding) map[string][]string {
	aliases := map[string][]string{}
	for _, name := range b.Skills {
		alias := langflowToolAlias("skill:"+name, "read_skill")
		aliases["read_skill"] = append(aliases["read_skill"], alias)
		aliases["read_file"] = append(aliases["read_file"], alias)
		if b.AllowSkillScripts {
			aliases["execute_skill_script"] = append(aliases["execute_skill_script"], langflowToolAlias("skill:"+name, "execute_skill_script"))
		}
	}
	for _, id := range b.KnowledgeBases {
		aliases["knowledge_search"] = append(aliases["knowledge_search"], langflowToolAlias("knowledge:"+id, "knowledge_search"))
	}
	for _, tool := range b.Tools {
		aliases[tool.Name] = append(aliases[tool.Name], langflowToolAlias(tool.ServiceID, tool.Name))
	}
	return aliases
}
