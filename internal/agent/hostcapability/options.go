// Package hostcapability describes authorized host services without an orchestration dependency.
package hostcapability

import (
	"context"
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

type Invocation struct {
	SessionID, MessageID, RunID, NodeKey, ToolCallID, ServiceID, Tool string
	Args                                                              map[string]any
}
type ToolDescriptor struct {
	ServiceID   string          `json:"service_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type SkillDescriptor struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	RequiresTools []string `json:"requires_tools,omitempty"`
}
type SkillInvocation struct {
	SessionID, MessageID, RunID, NodeKey, Name, FilePath string
	Execute                                              bool
	Args                                                 []string
	Stdin                                                string
	Offset, Limit                                        int
	MaxBytes                                             int64
}
type Options struct {
	Config         *types.AgentConfig
	Events         *event.EventBus
	FlowURL        string
	LocalAutoLogin bool
	ServiceAPIKey  string
	RequireRelease bool
	VerifyRelease  func(context.Context) error
	ResolveModel   func(context.Context, string) (chat.Chat, error)
	Revalidate     func(context.Context) (types.SearchTargets, error)
	Retrieve       func(context.Context, types.SearchTargets, []string) ([]*types.SearchResult, error)
	FilterEvidence func(context.Context, types.SearchTargets, []*types.SearchResult) ([]*types.SearchResult, error)
	CallTool       func(context.Context, Invocation) (map[string]any, error)
	BeforePublish  func(context.Context) error
	Tools          []ToolDescriptor
	Skills         []SkillDescriptor
	CallSkill      func(context.Context, SkillInvocation) (map[string]any, error)
	Cleanup        func(context.Context)
}
