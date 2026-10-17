package session

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

var executionProcessID = uuid.NewString()

// Both embedded engines are process-local. Losing the live owner ends the
// turn; replay must never start another engine or repeat business tools.
func (h *Handler) finishAbandonedEinoRun(ctx context.Context, message *types.Message, events []interfaces.StreamEvent) (bool, error) {
	if message.IsCompleted || message.RunState == types.MessageRunStateAwaitingInput {
		return false, nil
	}
	// Persisting the row precedes publication of its live marker. A message
	// owned by this process may be in that window; absence is not a crash.
	if message.ExecutionContext.ExecutionOwner == executionProcessID ||
		(message.ExecutionContext.ExecutionOwner == "" && len(events) == 0) {
		return false, nil
	}
	definition, _ := h.config.EinoDefinition(message.AgentID)
	engine := message.ExecutionContext.ExecutionEngine
	if message.ExecutionContext.ExecutionDefinition != "" {
		definition = message.ExecutionContext.ExecutionDefinition
	}
	var runID, nodeKey string
	for _, ev := range events {
		if ev.Type == types.ResponseTypeComplete {
			return false, nil
		}
		if d, _ := ev.Data["definition_version"].(string); d != "" {
			_, known := einodef.Lookup(d)
			if !known && ev.Data["execution_engine"] != "langflow" {
				continue
			}
			definition = d
			if known {
				engine = "eino"
			} else {
				engine = "langflow"
			}
			runID, _ = ev.Data["run_id"].(string)
			nodeKey, _ = ev.Data["node_key"].(string)
		}
	}
	if _, known := einodef.Lookup(definition); known {
		engine = "eino"
	}
	if engine != "eino" && engine != "langflow" {
		return false, nil
	}
	liveID, _, err := h.streamManager.GetLiveRun(ctx, message.SessionID)
	if err != nil {
		return false, err
	}
	if liveID == message.ID {
		return false, nil
	}
	// Completion persists before its live marker is cleared. Re-read to avoid
	// overwriting a result that finished between the history read and lookup.
	fresh, err := h.messageService.GetMessage(ctx, message.SessionID, message.ID)
	if err != nil {
		return false, err
	}
	if fresh == nil {
		return false, nil
	}
	*message = *fresh
	if fresh.IsCompleted || fresh.RunState == types.MessageRunStateAwaitingInput || fresh.ExecutionContext.ExecutionOwner == executionProcessID {
		return false, nil
	}
	message.Content = "任务未完成：执行进程已中断，无法继续本次任务，请重新发起。"
	message.IsCompleted = true
	message.UpdatedAt = time.Now()
	message.KnowledgeReferences = nil
	diagnostic := types.NewExecutionDiagnostic(engine, "execution", "execution_process_interrupted", nil)
	diagnostic.Definition, diagnostic.RunID = definition, runID
	message.ExecutionContext.ExecutionDiagnostics = append(message.ExecutionContext.ExecutionDiagnostics, diagnostic)
	if len(message.AgentSteps) == 0 && runID != "" {
		message.AgentSteps = []types.AgentStep{{Thought: "执行进程已中断", RunID: runID, NodeKey: nodeKey, DefinitionVersion: definition, Timestamp: time.Now()}}
	}
	if err := h.messageService.UpdateMessage(ctx, message); err != nil {
		return false, err
	}
	for _, ev := range []interfaces.StreamEvent{
		{ID: message.ID + ":interrupted", Type: types.ResponseTypeError, Content: message.Content, Done: true, Timestamp: time.Now(), Data: map[string]interface{}{"stage": "agent_execution", "definition_version": definition, "error": message.Content}},
		{ID: message.ID + ":interrupted-complete", Type: types.ResponseTypeComplete, Done: true, Timestamp: time.Now()},
	} {
		if err := h.streamManager.AppendEvent(ctx, message.SessionID, message.ID, ev); err != nil {
			return false, err
		}
	}
	return true, nil
}
