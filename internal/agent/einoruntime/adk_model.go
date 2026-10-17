package einoruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"reflect"
	"strings"
	"time"
)

// The adapter uses the host's complete native stream reader. ADK receives one
// completed message; cancellation, finish validation and usage stay in the host.
type taskChatModel struct {
	run   *run
	tools []*schema.ToolInfo
	agent string
}

var _ model.ToolCallingChatModel = (*taskChatModel)(nil)

func (a *taskChatModel) WithTools(in []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	copy := *a
	copy.tools = append([]*schema.ToolInfo(nil), in...)
	return &copy, nil
}
func (a *taskChatModel) Generate(ctx context.Context, in []*schema.Message, os ...model.Option) (*schema.Message, error) {
	for _, o := range os {
		f := reflect.ValueOf(o).FieldByName("implSpecificOptFn")
		if f.IsValid() && !f.IsNil() {
			return nil, fmt.Errorf("ADK implementation model options unsupported")
		}
	}
	common := model.GetCommonOptions(nil, os...)
	if common.Model != nil || common.Stop != nil || common.AllowedToolNames != nil || common.DeferredTools != nil || common.ToolSearchTool != nil || common.AgenticToolChoice != nil {
		return nil, fmt.Errorf("ADK model option unsupported")
	}
	opts := &chat.ChatOptions{Temperature: 0, MaxTokens: 1024, MaxCompletionTokens: 1024, Thinking: a.run.engine.opts.Config.Thinking}
	name := strings.ToLower(a.run.engine.opts.Model.GetModelName())
	if strings.HasPrefix(name, "deepseek-v4") || name == "deepseek-flash" {
		opts.ThinkingControl = "thinking_type"
	}
	if common.Temperature != nil {
		opts.Temperature = float64(*common.Temperature)
	}
	if common.TopP != nil {
		opts.TopP = float64(*common.TopP)
	}
	if common.MaxTokens != nil {
		if *common.MaxTokens <= 0 || *common.MaxTokens > 1024 {
			return nil, fmt.Errorf("ADK token budget unsupported")
		}
		opts.MaxTokens = *common.MaxTokens
		opts.MaxCompletionTokens = *common.MaxTokens
	}
	if common.ToolChoice != nil {
		switch string(*common.ToolChoice) {
		case "allowed":
			opts.ToolChoice = "auto"
		case "forced":
			opts.ToolChoice = "required"
		case "forbidden":
			opts.ToolChoice = "none"
		default:
			return nil, fmt.Errorf("ADK tool choice unsupported")
		}
	}
	infos := a.tools
	if common.Tools != nil {
		infos = common.Tools
	}
	allowed := map[string]bool{}
	for _, info := range infos {
		if info == nil || info.Name == "" || allowed[info.Name] {
			return nil, fmt.Errorf("invalid ADK tool definition")
		}
		allowed[info.Name] = true
		p := json.RawMessage(`{"type":"object"}`)
		if info.ParamsOneOf != nil {
			s, err := info.ParamsOneOf.ToJSONSchema()
			if err != nil {
				return nil, err
			}
			p, err = json.Marshal(s)
			if err != nil {
				return nil, err
			}
		}
		opts.Tools = append(opts.Tools, chat.Tool{Type: "function", Function: chat.FunctionDef{Name: info.Name, Description: info.Desc, Parameters: p}})
	}
	msgs := make([]chat.Message, 0, len(in))
	for _, m := range in {
		if m == nil || len(m.MultiContent)+len(m.UserInputMultiContent)+len(m.AssistantGenMultiContent) > 0 {
			return nil, fmt.Errorf("ADK supports text messages only")
		}
		n := chat.Message{Role: string(m.Role), Content: m.Content, Name: m.Name, ToolCallID: m.ToolCallID, ReasoningContent: m.ReasoningContent}
		if m.Role == schema.Tool && m.ToolCallID != "" {
			if err := a.run.completeDelegationCall(ctx, m.ToolCallID); err != nil {
				return nil, err
			}
		}
		for _, tc := range m.ToolCalls {
			var md types.ToolCallMetadata
			if v, ok := tc.Extra["teknowra_provider_metadata"]; ok {
				raw, err := json.Marshal(v)
				if err != nil {
					return nil, err
				}
				if err = json.Unmarshal(raw, &md); err != nil {
					return nil, err
				}
			}
			n.ToolCalls = append(n.ToolCalls, chat.ToolCall{ID: tc.ID, Type: tc.Type, Function: chat.FunctionCall{Name: tc.Function.Name, Arguments: tc.Function.Arguments}, ProviderMetadata: md})
		}
		msgs = append(msgs, n)
	}
	var response *ModelResponse
	_, err := stage(ctx, fmt.Sprintf("%s/model/%d", a.agent, a.run.calls), a.agent+"：选择下一步", func(r *run) error {
		if r.calls >= 16 {
			return fmt.Errorf("task model call budget exceeded")
		}
		r.calls++
		child, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		var e error
		response, e = ReadModel(child, r.engine.opts.Model, msgs, opts)
		if e != nil {
			return e
		}
		r.state.TurnUsage.Accumulate(response.Usage)
		r.usageComplete = r.usageComplete && response.UsageReported
		seen := map[string]bool{}
		for _, tc := range response.ToolCalls {
			if !allowed[tc.Function.Name] || tc.ID == "" || seen[tc.ID] || !json.Valid([]byte(tc.Function.Arguments)) {
				return fmt.Errorf("invalid ADK tool call")
			}
			seen[tc.ID] = true
			var args map[string]json.RawMessage
			if json.Unmarshal([]byte(tc.Function.Arguments), &args) != nil || args == nil || len(args) > 0 {
				return fmt.Errorf("ADK expert tools do not accept resource arguments")
			}
			for _, step := range r.state.RoundSteps {
				for _, call := range step.ToolCalls {
					if call.ID == tc.ID {
						return fmt.Errorf("ADK tool call id reused")
					}
				}
			}
		}
		for _, tc := range response.ToolCalls {
			idx := len(r.state.RoundSteps) - 1
			r.state.RoundSteps[idx].ToolCalls = append(r.state.RoundSteps[idx].ToolCalls, types.ToolCall{ID: tc.ID, Name: tc.Function.Name, Args: map[string]any{}, Target: &types.ToolCallTarget{Name: tc.Function.Name, ToolName: tc.Function.Name}, Result: &types.ToolResult{Success: false, Error: "专家执行未完成"}})
			if e := r.engine.emit(ctx, r, event.EventAgentToolCall, tc.ID+":call", event.AgentToolCallData{ToolCallID: tc.ID, ToolName: tc.Function.Name, Iteration: idx, Hint: "调用已发布专家能力"}); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := &schema.Message{Role: schema.Assistant, Content: response.Content, ReasoningContent: response.ReasoningContent, ResponseMeta: &schema.ResponseMeta{FinishReason: response.FinishReason, Usage: &schema.TokenUsage{PromptTokens: response.Usage.PromptTokens, CompletionTokens: response.Usage.CompletionTokens, TotalTokens: response.Usage.TotalTokens}}}
	for i, tc := range response.ToolCalls {
		idx := i
		out.ToolCalls = append(out.ToolCalls, schema.ToolCall{Index: &idx, ID: tc.ID, Type: tc.Type, Function: schema.FunctionCall{Name: tc.Function.Name, Arguments: tc.Function.Arguments}, Extra: map[string]any{"teknowra_provider_metadata": tc.ProviderMetadata}})
	}
	return out, nil
}

func (r *run) completeDelegationCall(ctx context.Context, id string) error {
	for i := range r.state.RoundSteps {
		for j := range r.state.RoundSteps[i].ToolCalls {
			call := &r.state.RoundSteps[i].ToolCalls[j]
			if call.ID != id || call.Result == nil || call.Result.Success {
				continue
			}
			expert := call.Name
			if expert == "execute_contract" {
				expert = "contract_expert"
			}
			if expert == "execute_credit" {
				expert = "credit_expert"
			}
			if r.expertResults[expert] == "" {
				return fmt.Errorf("专家未实际完成，拒绝工具成功标记")
			}
			call.Result = &types.ToolResult{Success: true, Output: "专家只读结果已由平台保存"}
			return r.engine.emit(ctx, r, event.EventAgentToolResult, id+":result", event.AgentToolResultData{ToolCallID: id, ToolName: call.Name, Iteration: i, Success: true, Output: call.Result.Output})
		}
	}
	return nil
}
func (a *taskChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m, err := a.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{m}), nil
}
