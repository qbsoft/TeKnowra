package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowapi"
	"github.com/Tencent/WeKnora/internal/agent/langflowcatalog"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"os"
	"slices"
	"sort"
	"time"
)

type LangflowReleaseService struct {
	cfg     *config.Config
	catalog *LangflowCatalogService
	manager *langflowrelease.Manager
	runtime *langflowapi.Client
}

func releaseManager(cfg *config.Config) (*langflowrelease.Manager, *langflowapi.Client, config.LangflowCredentials, error) {
	creds, err := config.ReadLangflowCredentials(os.Getenv("TEKNOWRA_LANGFLOW_CREDENTIALS_FILE"))
	if err != nil {
		return nil, nil, creds, err
	}
	base := os.Getenv("TEKNOWRA_LANGFLOW_URL")
	if base == "" {
		base = "http://127.0.0.1:7861"
	}
	editor, err := langflowapi.New(base, creds.EditorAPIKey)
	if err != nil {
		return nil, nil, config.LangflowCredentials{}, err
	}
	runtime, err := langflowapi.New(base, creds.RuntimeAPIKey)
	if err != nil {
		return nil, nil, config.LangflowCredentials{}, err
	}
	return &langflowrelease.Manager{Store: cfg.LangflowReleases(), Editor: editor, Runtime: runtime, RuntimeOwnerID: creds.RuntimeOwnerID}, runtime, creds, nil
}
func NewLangflowReleaseService(cfg *config.Config, catalog *LangflowCatalogService) (*LangflowReleaseService, error) {
	s := &LangflowReleaseService{cfg: cfg, catalog: catalog}
	if cfg.LangflowReleases() == nil {
		return s, nil
	}
	if os.Getenv("TEKNOWRA_LANGFLOW_LOCAL_TEST") == "true" {
		return nil, fmt.Errorf("发布模式禁止Langflow测试自动登录")
	}
	var err error
	s.manager, s.runtime, _, err = releaseManager(cfg)
	if err == nil {
		s.manager.BeforeActivate = func(ctx context.Context, r langflowrelease.Release) error {
			catalog, err := s.catalog.Catalog(ctx, r.AgentID)
			if err != nil {
				return err
			}
			if err = completeReleaseCatalog(catalog, r.Binding); err != nil {
				return err
			}
			components, err := releaseBindingComponents(ctx, s.runtime, r.Binding)
			if err != nil {
				return err
			}
			digest, err := releaseCapabilityDigest(ctx, s.catalog.mcps, r.Binding, s.catalog.skillState)
			if err != nil {
				return err
			}
			return s.manager.Verify(ctx, r, components.Digest, digest)
		}
	}
	return s, err
}
func (s *LangflowReleaseService) Enabled() bool { return s != nil && s.manager != nil }
func releaseComponents(ctx context.Context, runtime *langflowapi.Client) (langflowrelease.Components, error) {
	root := os.Getenv("TEKNOWRA_LANGFLOW_COMPONENTS_ROOT")
	if root == "" {
		root = "integrations/langflow"
	}
	lib := os.Getenv("TEKNOWRA_LANGFLOW_LIBRARY_ROOT")
	if lib == "" {
		return langflowrelease.Components{}, fmt.Errorf("请配置本机Langflow库目录")
	}
	return langflowrelease.LocalComponents(ctx, runtime, root, lib)
}
func releaseBindingComponents(ctx context.Context, runtime *langflowapi.Client, b langflowdef.Binding) (langflowrelease.Components, error) {
	base, err := releaseComponents(ctx, runtime)
	if err != nil {
		return base, err
	}
	root := os.Getenv("TEKNOWRA_LANGFLOW_COMPONENTS_ROOT")
	if root == "" {
		root = "integrations/langflow"
	}
	if len(b.Skills) > 0 {
		base, err = langflowrelease.WithSkillComponent(base, root)
		if err != nil {
			return base, err
		}
	}
	if b.ReviewedExperts {
		return langflowrelease.WithReviewedExpertPolicy(base, root, b.ReviewedExpertPolicy)
	}
	return base, nil
}
func releaseCapabilityDigest(ctx context.Context, mcps interfaces.MCPServiceService, b langflowdef.Binding, load func(context.Context, langflowdef.Binding) (langflowSkillState, error)) (string, error) {
	tools, err := releaseTools(ctx, mcps, b)
	if err != nil {
		return "", err
	}
	digest, err := releaseToolDigest(tools)
	if err != nil || len(b.Skills) == 0 {
		return digest, err
	}
	skills, err := load(ctx, b)
	if err != nil {
		return "", err
	}
	raw, _ := json.Marshal(map[string]any{"tools": digest, "skills": skills.Digest})
	return langflowrelease.Digest(raw)
}
func releaseToolDigest(tools []hostcapability.ToolDescriptor) (string, error) {
	tools = slices.Clone(tools)
	if tools == nil {
		tools = []hostcapability.ToolDescriptor{}
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].ServiceID+"/"+tools[i].Name < tools[j].ServiceID+"/"+tools[j].Name
	})
	raw, err := json.Marshal(tools)
	if err != nil {
		return "", err
	}
	return langflowrelease.Digest(raw)
}
func releaseTools(ctx context.Context, mcps interfaces.MCPServiceService, b langflowdef.Binding) ([]hostcapability.ToolDescriptor, error) {
	result := []hostcapability.ToolDescriptor{}
	if len(b.Tools) == 0 {
		return result, nil
	}
	metadata, ok := mcps.(interfaces.MCPMetadataService)
	if !ok {
		return nil, fmt.Errorf("工具目录未就绪")
	}
	for _, binding := range b.Tools {
		svc, err := mcps.GetMCPServiceByID(ctx, b.WorkspaceID, binding.ServiceID)
		if err != nil || svc == nil || !svc.Enabled || (!svc.IsBuiltin && svc.TenantID != b.WorkspaceID) {
			return nil, fmt.Errorf("绑定工具服务不可用")
		}
		snapshot, err := metadata.GetMCPMetadata(ctx, b.WorkspaceID, binding.ServiceID)
		if err != nil || snapshot == nil || snapshot.Stale {
			return nil, fmt.Errorf("请同步绑定工具目录")
		}
		found := false
		for _, tool := range snapshot.Tools {
			if tool != nil && tool.Name == binding.Name {
				result = append(result, hostcapability.ToolDescriptor{ServiceID: binding.ServiceID, Name: tool.Name, Description: tool.Description, InputSchema: slices.Clone(tool.InputSchema)})
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("绑定工具已不可用")
		}
	}
	return result, nil
}
func completeReleaseCatalog(c langflowcatalog.Catalog, b langflowdef.Binding) error {
	if err := b.ValidateScriptPolicies(); err != nil {
		return err
	}
	if c.AllowSessionAttachments != b.AllowSessionAttachments {
		return fmt.Errorf("会话附件策略与待发布绑定不一致")
	}
	for _, name := range b.Skills {
		found := false
		for _, skill := range c.Skills {
			if skill.ID == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("发布技能不在当前可用与Agent授权范围")
		}
	}
	has := func(rows []langflowcatalog.Resource, id string) bool {
		for _, row := range rows {
			if row.ID == id {
				return true
			}
		}
		return false
	}
	for _, id := range b.Models {
		if !has(c.Models, id) {
			return fmt.Errorf("发布模型不在当前可用范围")
		}
	}
	for _, id := range b.KnowledgeBases {
		if !has(c.KnowledgeBases, id) {
			return fmt.Errorf("发布知识库不在当前授权范围")
		}
	}
	for _, tool := range b.Tools {
		found := false
		for _, svc := range c.Services {
			if svc.ID == tool.ServiceID {
				for _, row := range svc.Tools {
					if row.Name == tool.Name {
						found = true
					}
				}
			}
		}
		if !found {
			return fmt.Errorf("发布工具不在当前Agent授权范围")
		}
	}
	return nil
}
func (s *LangflowReleaseService) authorize(ctx context.Context, agent string) (langflowdef.Binding, error) {
	if !s.Enabled() {
		return langflowdef.Binding{}, fmt.Errorf("固定发布未启用")
	}
	catalog, err := s.catalog.Catalog(ctx, agent)
	if err != nil {
		return langflowdef.Binding{}, err
	}
	b, ok := s.cfg.DraftLangflowBinding(agent)
	if !ok {
		return b, fmt.Errorf("未知绑定")
	}
	// Capture the current explicit agent policy in each immutable publication.
	b.AllowSessionAttachments = catalog.AllowSessionAttachments
	return b, completeReleaseCatalog(catalog, b)
}
func (s *LangflowReleaseService) Publish(ctx context.Context, agent, label string) (langflowrelease.Release, error) {
	b, err := s.authorize(ctx, agent)
	if err != nil {
		return langflowrelease.Release{}, err
	}
	call, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	flow, err := s.manager.Editor.GetFlow(call, b.FlowID)
	if err != nil {
		return langflowrelease.Release{}, err
	}
	b.ReviewedExperts = langflowrelease.GraphUsesReviewedExperts(flow.Data)
	b.ReviewedExpertPolicy = langflowrelease.GraphReviewedExpertPolicy(flow.Data)
	components, err := releaseBindingComponents(call, s.runtime, b)
	if err != nil {
		return langflowrelease.Release{}, err
	}
	digest, err := releaseCapabilityDigest(call, s.catalog.mcps, b, s.catalog.skillState)
	if err != nil {
		return langflowrelease.Release{}, err
	}
	return s.manager.Publish(call, b, label, components, digest)
}
func (s *LangflowReleaseService) List(ctx context.Context, agent string) ([]langflowrelease.Release, string, error) {
	if _, err := s.authorize(ctx, agent); err != nil {
		return nil, "", err
	}
	active, _ := s.manager.Store.Active(agent)
	return s.manager.Store.List(agent), active.ID, nil
}
func (s *LangflowReleaseService) Activate(ctx context.Context, agent, id string) error {
	b, err := s.authorize(ctx, agent)
	if err != nil {
		return err
	}
	release, ok := s.manager.Store.Get(agent, id)
	if !ok || release.WorkspaceID != b.WorkspaceID {
		return fmt.Errorf("版本不属于当前智能体和工作空间")
	}
	catalog, err := s.catalog.Catalog(ctx, agent)
	if err != nil {
		return err
	}
	if err = completeReleaseCatalog(catalog, release.Binding); err != nil {
		return err
	}
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	components, err := releaseBindingComponents(call, s.runtime, release.Binding)
	if err != nil {
		return err
	}
	digest, err := releaseCapabilityDigest(call, s.catalog.mcps, release.Binding, s.catalog.skillState)
	if err != nil {
		return err
	}
	return s.manager.Activate(call, agent, id, components.Digest, digest)
}
