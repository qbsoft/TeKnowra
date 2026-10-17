package langflowverify

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

// These engines retain their own bounded in-process OAuth wait. They do not
// produce native agent snapshots, so enabling native suspension is a no-op.
func (e *Engine) EnableSuspension(json.RawMessage) {}
func (e *Engine) SetInteractive(bool)              {}

// Never restart Execute to emulate resume: that could repeat business calls.
func (e *Engine) Resume(context.Context, string, string, []chat.Message, []byte, *types.AgentPrompt) (*types.AgentState, error) {
	return nil, fmt.Errorf("Langflow执行器不支持原生智能体快照恢复，请重新发起任务")
}
