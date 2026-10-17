// Package langflowruntime executes bound Langflow graphs through host capabilities.
// It reuses host capability callbacks; it never constructs employee identities
// from Langflow inputs and does not execute an Eino graph.
package langflowruntime

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/api"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
)

type bridge struct {
	ctx                                context.Context
	token, sessionID, messageID, runID string
	opts                               hostcapability.Options
	binding                            langflowdef.Binding
	mu                                 sync.Mutex
	state                              types.AgentState
	refs                               []*types.SearchResult
	history                            []chat.Message

	modelCalls, toolCalls int
	failure               error
}

func (b *bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The listener is unique to this invocation and closes when Execute returns.
	if b.ctx.Err() != nil || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+b.token)) != 1 {
		http.Error(w, "task capability unavailable", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	ctx, cancel := context.WithCancel(b.ctx)
	defer cancel()
	stop := context.AfterFunc(r.Context(), cancel)
	defer stop()
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var err error
	switch r.URL.Path {
	case "/knowledge":
		err = b.knowledge(ctx, w, r)
	case "/tool":
		err = b.tool(ctx, w, r)
	case "/skill":
		err = b.skill(ctx, w, r)
	case "/v1/chat/completions":
		err = b.model(ctx, w, r)
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		if len(b.state.ExecutionDiagnostics) == 0 {
			d := types.NewExecutionDiagnostic("langflow", r.URL.Path, types.ExecutionFailureCode(err), nil)
			d.Definition = bindingVersion(b.binding)
			d.Implementation = b.binding.GraphDigest
			d.RunID = b.runID
			b.state.ExecutionDiagnostics = append(b.state.ExecutionDiagnostics, d)
		}
		detail := strings.ReplaceAll(err.Error(), b.token, "[task capability]")
		detail = regexp.MustCompile(`sk-[A-Za-z0-9_-]+|[0-9a-f]{64}`).ReplaceAllString(detail, "[redacted]")
		if len(detail) > 500 {
			detail = detail[:500]
		}
		b.failure = fmt.Errorf("%s: %s (%s)", r.URL.Path, failureKind(err), detail)
		// Do not put provider responses, capabilities or business data in logs/errors.
		http.Error(w, "TeKnowra task capability failed: "+b.failure.Error(), http.StatusBadGateway)
	}
}

func failureKind(err error) string {
	text := err.Error()
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "model_or_task_timeout"
	case errors.Is(err, context.Canceled):
		return "task_canceled"
	case strings.Contains(text, "finish=length"):
		return "model_output_truncated"
	case strings.Contains(text, "model stream ended"):
		return "model_stream_incomplete"
	case strings.Contains(text, "json: unknown field"):
		return "unsupported_request_field"
	case strings.Contains(text, "tool choice"):
		return "unsupported_tool_choice"
	case strings.Contains(text, "budget exceeded"):
		return "call_budget_exceeded"
	case strings.Contains(text, "model stream:"):
		return "provider_stream_error"
	default:
		return safeError(err)
	}
}

func safeError(err error) string {
	if err == context.Canceled || err == context.DeadlineExceeded {
		return "task stopped"
	}
	return "permission, configuration or service check failed"
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("single JSON object required")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}
func (b *bridge) emit(ctx context.Context, kind event.EventType, key string, data any) error {
	if b.opts.Events == nil {
		return nil
	}
	return b.opts.Events.Emit(ctx, event.Event{ID: b.runID + ":" + key, Type: kind, SessionID: b.sessionID, Data: data})
}
func (b *bridge) record(ctx context.Context, name string, fn func() (any, error)) (any, error) {
	b.toolCalls++
	if b.toolCalls > 16 {
		return nil, fmt.Errorf("tool budget exceeded")
	}
	idx := len(b.state.RoundSteps)
	id := uuid.NewString()
	start := time.Now()
	b.state.RoundSteps = append(b.state.RoundSteps, types.AgentStep{ImplementationVersion: b.binding.GraphDigest, Iteration: idx, Thought: "Langflow：" + name, Timestamp: start, RunID: b.runID, NodeKey: name, DefinitionVersion: bindingVersion(b.binding)})
	if err := b.emit(ctx, event.EventAgentToolCall, id+":call", event.AgentToolCallData{ToolCallID: id, ToolName: name, Iteration: idx, Hint: "通过当前员工的 TeKnowra 能力执行"}); err != nil {
		return nil, err
	}
	out, err := fn()
	result := &types.ToolResult{Success: err == nil, Output: "受权限约束的调用完成", Data: map[string]any{"execution_engine": "langflow"}}
	if name == "knowledge_search" && err == nil {
		if data, ok := out.(map[string]any); ok {
			result.Data["results"] = data["results"]
		}
	}
	if err != nil {
		result.Error = safeError(err)
		result.Output = ""
	}
	b.state.RoundSteps[idx].ToolCalls = []types.ToolCall{{ID: id, Name: name, Result: result, Duration: time.Since(start).Milliseconds()}}
	if e := b.emit(ctx, event.EventAgentToolResult, id+":result", event.AgentToolResultData{ToolCallID: id, ToolName: name, Success: err == nil, Output: result.Output, Error: result.Error, Duration: time.Since(start).Milliseconds(), Iteration: idx, Data: result.Data}); e != nil {
		return nil, e
	}
	return out, err
}
func (b *bridge) knowledge(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Query           string `json:"query"`
		KnowledgeBaseID string `json:"knowledge_base_id"`
		NodeKey         string `json:"node_key,omitempty"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Query) == "" || len(in.Query) > 8000 {
		return fmt.Errorf("invalid query")
	}
	if !slices.Contains(b.binding.KnowledgeBases, in.KnowledgeBaseID) || b.opts.Retrieve == nil || b.opts.FilterEvidence == nil {
		return fmt.Errorf("knowledge capability not bound")
	}
	out, err := b.record(ctx, "knowledge_search", func() (any, error) {
		scope, err := b.opts.Revalidate(ctx)
		if err != nil {
			return nil, err
		}
		if in.KnowledgeBaseID != "" {
			selected := types.SearchTargets{}
			for _, s := range scope {
				if s != nil && s.KnowledgeBaseID == in.KnowledgeBaseID {
					selected = append(selected, s)
				}
			}
			if len(selected) == 0 {
				return nil, fmt.Errorf("knowledge base is not permitted")
			}
			scope = selected
		}
		refs, err := b.opts.Retrieve(ctx, scope, []string{in.Query})
		if err != nil {
			return nil, err
		}
		scope, err = b.opts.Revalidate(ctx)
		if err != nil {
			return nil, err
		}
		refs, err = b.opts.FilterEvidence(ctx, scope, refs)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, ref := range b.refs {
			seen[ref.ID] = true
		}
		rows := []map[string]any{}
		for _, ref := range refs {
			if ref == nil || ref.ID == "" {
				continue
			}
			if !seen[ref.ID] {
				b.refs = append(b.refs, ref)
				seen[ref.ID] = true
			}
			rows = append(rows, map[string]any{"source_id": ref.ID, "knowledge_id": ref.KnowledgeID, "title": ref.KnowledgeTitle, "content": ref.Content})
		}
		return map[string]any{"results": rows}, nil
	})
	if err != nil {
		return err
	}
	return writeJSON(w, out)
}
func (b *bridge) invoke(ctx context.Context, service, tool, node string, args map[string]any) (map[string]any, error) {
	if _, err := b.opts.Revalidate(ctx); err != nil {
		return nil, err
	}
	return b.opts.CallTool(ctx, hostcapability.Invocation{SessionID: b.sessionID, MessageID: b.messageID, RunID: b.runID, NodeKey: node, ToolCallID: uuid.NewString(), ServiceID: service, Tool: tool, Args: args})
}
func (b *bridge) tool(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Tool      string         `json:"tool"`
		Args      map[string]any `json:"args"`
		ServiceID string         `json:"service_id"`
		NodeKey   string         `json:"node_key,omitempty"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if !b.binding.AllowsTool(in.ServiceID, in.Tool) || b.opts.CallTool == nil {
		return fmt.Errorf("tool or service not published")
	}
	out, err := b.record(ctx, in.Tool, func() (any, error) {
		return b.invoke(ctx, in.ServiceID, in.Tool, in.NodeKey, in.Args)
	})
	if err != nil {
		return err
	}
	return writeJSON(w, out)
}

func (b *bridge) model(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	// The remote caller can choose prompts/tool schemas, not the provider identity.
	var in struct {
		Model               string            `json:"model"`
		Messages            []json.RawMessage `json:"messages"`
		Tools               []chat.Tool       `json:"tools"`
		Temperature         float64           `json:"temperature"`
		TopP                float64           `json:"top_p"`
		Stream              bool              `json:"stream"`
		MaxTokens           int               `json:"max_tokens"`
		MaxCompletionTokens int               `json:"max_completion_tokens"`
		ToolChoice          json.RawMessage   `json:"tool_choice"`
		ParallelToolCalls   *bool             `json:"parallel_tool_calls"`
		StreamOptions       json.RawMessage   `json:"stream_options"`
		Seed                int               `json:"seed"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if len(in.Messages) == 0 || len(in.Messages) > 80 || len(in.Tools) > 16 {
		return fmt.Errorf("model input budget exceeded")
	}
	if !slices.Contains(b.binding.Models, in.Model) || b.opts.ResolveModel == nil {
		return fmt.Errorf("model is not the host-authorized binding")
	}
	if _, err := b.opts.Revalidate(ctx); err != nil {
		return err
	}
	model, err := b.opts.ResolveModel(ctx, in.Model)
	if err != nil || model == nil {
		return fmt.Errorf("authorized model unavailable")
	}
	b.modelCalls++
	if b.modelCalls > 12 {
		return fmt.Errorf("model call budget exceeded")
	}
	messages := []chat.Message{}
	for _, raw := range in.Messages {
		var m struct {
			Role             string          `json:"role"`
			Content          json.RawMessage `json:"content"`
			Name             string          `json:"name"`
			ToolCallID       string          `json:"tool_call_id"`
			ToolCalls        []chat.ToolCall `json:"tool_calls"`
			ReasoningContent string          `json:"reasoning_content"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		content := ""
		if len(m.Content) > 0 && string(m.Content) != "null" {
			if err := json.Unmarshal(m.Content, &content); err != nil {
				var parts []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}
				if err = json.Unmarshal(m.Content, &parts); err != nil {
					return err
				}
				for _, p := range parts {
					if p.Type != "text" {
						return fmt.Errorf("only text input is supported")
					}
					content += p.Text
				}
			}
		}
		messages = append(messages, chat.Message{Role: m.Role, Content: content, Name: m.Name, ToolCallID: m.ToolCallID, ToolCalls: m.ToolCalls, ReasoningContent: m.ReasoningContent})
	}
	messages = withHistory(messages, b.history)
	var pinned []string
	if b.opts.Config != nil {
		pinned = b.opts.Config.PinnedSkillNames
	}
	messages = withSkillMetadata(messages, b.opts.Skills, pinned)
	choice := ""
	if len(in.ToolChoice) > 0 && string(in.ToolChoice) != "null" {
		if err := json.Unmarshal(in.ToolChoice, &choice); err != nil {
			return fmt.Errorf("only simple tool choice supported")
		}
	}
	child, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()
	modelOptions := &chat.ChatOptions{Temperature: in.Temperature, TopP: in.TopP, Seed: in.Seed, MaxTokens: min(max(in.MaxTokens, 1024), 8192), MaxCompletionTokens: in.MaxCompletionTokens, Tools: in.Tools, ToolChoice: choice, ParallelToolCalls: in.ParallelToolCalls}
	// Reasoning is a saved host setting, not a provider default or a canvas
	// override. In particular DeepSeek v4 otherwise spends the bounded output
	// budget on reasoning and may produce no answer at all.
	if b.opts.Config != nil {
		modelOptions.Thinking = b.opts.Config.Thinking
		modelOptions.ReasoningEffort = api.ReasoningEffort(b.opts.Config.ReasoningEffort)
	}
	modelName := strings.ToLower(model.GetModelName())
	if strings.HasPrefix(modelName, "deepseek-v4") || modelName == "deepseek-flash" {
		modelOptions.ThinkingControl = "thinking_type"
		if modelOptions.Thinking == nil {
			disabled := false
			modelOptions.Thinking = &disabled
		}
	}
	response, err := hostcapability.ReadModel(child, model, messages, modelOptions)
	// Retry only a transport EOF, before a complete model response has reached
	// Langflow. No tool has been executed by this model request. Permission,
	// provider rejection, truncation and business-tool failures never retry here.
	if errors.Is(err, io.EOF) && child.Err() == nil && b.modelCalls < 12 {
		b.modelCalls++
		if _, checkErr := b.opts.Revalidate(child); checkErr != nil {
			return checkErr
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-child.Done():
			timer.Stop()
			return child.Err()
		case <-timer.C:
		}
		response, err = hostcapability.ReadModel(child, model, messages, modelOptions)
	}
	if err != nil {
		return err
	}
	b.state.TurnUsage.Accumulate(response.Usage)
	if b.binding.ReviewedExperts {
		for _, m := range messages {
			if strings.Contains(m.Content, "审查事实复核：") || strings.Contains(m.Content, "摘要事实复核：") {
				if d := types.AuditDiagnostic("langflow", "fact_review", response.Content); d != nil {
					d.Definition = bindingVersion(b.binding)
					d.Implementation = b.binding.GraphDigest
					d.RunID = b.runID
					b.state.ExecutionDiagnostics = append(b.state.ExecutionDiagnostics, *d)
				}
				break
			}
		}
	}
	if _, err := b.opts.Revalidate(ctx); err != nil {
		return err
	}
	message := map[string]any{"role": "assistant", "content": response.Content}
	if response.ReasoningContent != "" {
		message["reasoning_content"] = response.ReasoningContent
	}
	if len(response.ToolCalls) > 0 {
		message["tool_calls"] = response.ToolCalls
	}
	id := "chatcmpl-" + uuid.NewString()
	if !in.Stream {
		return writeJSON(w, map[string]any{"id": id, "object": "chat.completion", "created": time.Now().Unix(), "model": model.GetModelName(), "choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": response.FinishReason}}, "usage": response.Usage})
	}
	// This spike buffers the host stream, then emits protocol-compatible SSE.
	w.Header().Set("Content-Type", "text/event-stream")
	calls := []map[string]any{}
	for i, c := range response.ToolCalls {
		calls = append(calls, map[string]any{"index": i, "id": c.ID, "type": c.Type, "function": c.Function})
	}
	if len(calls) > 0 {
		message["tool_calls"] = calls
	}
	chunk := map[string]any{"id": id, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": model.GetModelName(), "choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": nil}}}
	raw, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", raw)
	chunk["choices"] = []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": response.FinishReason}}
	chunk["usage"] = response.Usage
	raw, _ = json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", raw)
	return nil
}
