package langflowrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowapi"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/google/uuid"
	"strings"
	"sync"
	"unicode"
)

type FlowClient interface {
	GetFlow(context.Context, string) (langflowapi.Flow, error)
	CreateFlow(context.Context, string, json.RawMessage) (langflowapi.Flow, error)
}
type Components struct {
	Digest   string
	Approved map[string]string
}
type Manager struct {
	draftMu         sync.Mutex
	Store           *Store
	Editor, Runtime FlowClient
	RuntimeOwnerID  string
	BeforeActivate  func(context.Context, Release) error
	mu              sync.Mutex
}

func (m *Manager) LockDraftSetup()   { m.draftMu.Lock() }
func (m *Manager) UnlockDraftSetup() { m.draftMu.Unlock() }

func (m *Manager) Publish(ctx context.Context, b langflowdef.Binding, label string, components Components, toolsDigest string) (Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	label = strings.TrimSpace(label)
	if label == "" || len([]rune(label)) > 64 || strings.IndexFunc(label, unicode.IsControl) >= 0 {
		return Release{}, fmt.Errorf("发布名称需要1—64个字符")
	}
	draft, err := m.Editor.GetFlow(ctx, b.FlowID)
	if err != nil {
		return Release{}, err
	}
	if draft.UserID == m.RuntimeOwnerID {
		return Release{}, fmt.Errorf("编辑身份与运行身份未隔离")
	}
	if err = ValidateGraph(draft.Data, b, components.Approved); err != nil {
		return Release{}, err
	}
	id := uuid.NewString()
	runtime, err := m.Runtime.CreateFlow(ctx, "TeKnowra release "+id, draft.Data)
	if err != nil {
		return Release{}, err
	}
	confirmed, err := m.Runtime.GetFlow(ctx, runtime.ID)
	if err != nil {
		return Release{}, err
	}
	if confirmed.UserID != m.RuntimeOwnerID {
		return Release{}, fmt.Errorf("运行工件所有者不符")
	}
	if err = ValidateGraph(confirmed.Data, b, components.Approved); err != nil {
		return Release{}, err
	}
	digest, err := Digest(confirmed.Data)
	if err != nil {
		return Release{}, err
	}
	sourceDigest, err := Digest(draft.Data)
	if err != nil || digest != sourceDigest {
		return Release{}, fmt.Errorf("导入改变了流程，未激活版本")
	}
	published := b.Clone()
	published.FlowID = runtime.ID
	published.ReleaseID = ""
	published.GraphDigest = ""
	published.RuntimeOwnerID = ""
	release := Release{ID: id, Label: label, AgentID: b.AgentID, WorkspaceID: b.WorkspaceID, EditorFlowID: b.FlowID, Binding: published, GraphDigest: digest, ToolsDigest: toolsDigest, ComponentsDigest: components.Digest, RuntimeOwnerID: m.RuntimeOwnerID}
	if m.BeforeActivate != nil {
		if err = m.BeforeActivate(ctx, release); err != nil {
			return Release{}, err
		}
	}
	if err = m.Store.Add(release); err != nil {
		return Release{}, err
	}
	if err = m.Store.Activate(b.AgentID, id); err != nil {
		return Release{}, err
	}
	saved, _ := m.Store.Get(b.AgentID, id)
	return saved, nil
}
func (m *Manager) Verify(ctx context.Context, r Release, componentsDigest, toolsDigest string) error {
	if r.RuntimeOwnerID != m.RuntimeOwnerID || r.ComponentsDigest != componentsDigest || r.ToolsDigest != toolsDigest {
		return fmt.Errorf("发布依赖已变化，请重新发布")
	}
	flow, err := m.Runtime.GetFlow(ctx, r.Binding.FlowID)
	if err != nil {
		return err
	}
	digest, err := Digest(flow.Data)
	if err != nil || digest != r.GraphDigest || flow.UserID != r.RuntimeOwnerID {
		return fmt.Errorf("发布工件已变化，拒绝执行或激活")
	}
	return nil
}
func (m *Manager) Activate(ctx context.Context, agent, id, componentsDigest, toolsDigest string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.Store.Get(agent, id)
	if !ok {
		return fmt.Errorf("版本不属于该智能体")
	}
	if err := m.Verify(ctx, r, componentsDigest, toolsDigest); err != nil {
		return err
	}
	if m.BeforeActivate != nil {
		if err := m.BeforeActivate(ctx, r); err != nil {
			return err
		}
	}
	return m.Store.Activate(agent, id)
}
