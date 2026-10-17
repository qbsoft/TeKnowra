package session

import (
	"context"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"strings"
	"testing"
)

func TestLangflowPreExecutionFailureIsVisibleAndStored(t *testing.T) {
	for _, reason := range []string{"绑定技能已禁用、未安装或不在当前执行镜像中", "provider api_key=DO_NOT_DISCLOSE exception"} {
		capture := &einoStreamCapture{}
		message := &types.Message{ID: "m"}
		h := &AgentStreamHandler{ctx: context.Background(), assistantMessage: message, streamManager: capture, langflowExecution: true}
		if err := h.handleError(context.Background(), event.Event{Data: event.ErrorData{Error: reason, Stage: "agent_execution"}}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(message.Content, "任务未完成") || strings.Contains(message.Content, "DO_NOT_DISCLOSE") || len(capture.events) != 1 || capture.events[0].Content != message.Content {
			t.Fatalf("failure not safely visible/persisted: %+v", message)
		}
	}
}

func TestLangflowUnboundPageAgentFailureHasPersistentSetupHint(t *testing.T) {
	capture := &einoStreamCapture{}
	message := &types.Message{ID: "m"}
	h := &AgentStreamHandler{ctx: context.Background(), assistantMessage: message, streamManager: capture}
	err := h.handleError(context.Background(), event.Event{Data: event.ErrorData{
		Error: "Langflow流程尚未接入或发布，请先在智能体页面完成配置", Stage: "agent_execution",
		Extra: map[string]interface{}{"execution_engine": "langflow"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Content, "先在智能体页面") || !strings.Contains(message.Content, "任务未完成") || len(capture.events) != 1 || capture.events[0].Content != message.Content {
		t.Fatalf("unbound page agent failure not visible/persisted: %+v", message)
	}
}
