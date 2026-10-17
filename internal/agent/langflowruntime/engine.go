package langflowruntime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowapi"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"github.com/Tencent/WeKnora/internal/agent/runbudget"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

type Engine struct {
	opts    hostcapability.Options
	binding langflowdef.Binding
	steer   types.SteerSink
}

var _ interfaces.AgentEngine = (*Engine)(nil)

func bindingVersion(b langflowdef.Binding) string {
	if b.ReleaseID != "" {
		return b.ReleaseID
	}
	return "langflow-binding-v1"
}
func (e *Engine) RuntimeMetadata() map[string]any {
	return map[string]any{"definition_version": bindingVersion(e.binding), "release_id": e.binding.ReleaseID, "flow_id": e.binding.FlowID}
}

func New(binding langflowdef.Binding, opts hostcapability.Options) (*Engine, error) {
	if opts.RequireRelease && (binding.ReleaseID == "" || binding.GraphDigest == "" || binding.RuntimeOwnerID == "" || opts.ServiceAPIKey == "" || opts.VerifyRelease == nil || opts.LocalAutoLogin) {
		return nil, fmt.Errorf("Langflow请先发布固定版本并配置运行服务认证")
	}
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	if opts.ResolveModel == nil || opts.Revalidate == nil {
		return nil, fmt.Errorf("Langflow需要模型和身份授权能力")
	}
	if len(binding.KnowledgeBases) > 0 && (opts.Retrieve == nil || opts.FilterEvidence == nil) {
		return nil, fmt.Errorf("Langflow知识授权能力缺失")
	}
	if len(binding.Tools) > 0 && opts.CallTool == nil {
		return nil, fmt.Errorf("Langflow工具授权能力缺失")
	}
	if len(binding.Skills) > 0 && (opts.CallSkill == nil || len(opts.Skills) == 0 || !opts.RequireRelease) {
		return nil, fmt.Errorf("Langflow技能需要已发布绑定与宿主技能能力")
	}
	if opts.Events == nil {
		opts.Events = event.NewEventBus()
	}
	if opts.FlowURL == "" {
		opts.FlowURL = "http://127.0.0.1:7861"
	}
	u, err := url.Parse(opts.FlowURL)
	// Remote authenticated deployment is outside this milestone.
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return nil, fmt.Errorf("当前阶段仅支持受控本机Langflow地址")
	}
	return &Engine{opts: opts, binding: binding.Clone()}, nil
}
func (e *Engine) SetMemoryPrompt(string)                               {}
func (e *Engine) SetSteerSink(s types.SteerSink)                       { e.steer = s }
func (e *Engine) SetContextCheckpointSink(types.ContextCheckpointSink) {}

func (e *Engine) Execute(parent context.Context, sessionID, messageID, query string, history []chat.Message, images ...[]string) (*types.AgentState, error) {
	if e.opts.Cleanup != nil {
		defer e.opts.Cleanup(parent)
	}
	release, err := runbudget.Acquire(parent)
	if err != nil {
		return nil, err
	}
	defer release()
	if len(history) > 0 && !e.opts.RequireRelease {
		return nil, fmt.Errorf("Langflow多轮对话需要固定发布和服务认证")
	}
	conversation, err := conversationHistory(history)
	if err != nil {
		return nil, err
	}
	for _, v := range images {
		if len(v) > 0 {
			return nil, fmt.Errorf("Langflow当前接入仅接受文本")
		}
	}
	ctx, cancel := runbudget.New(parent, runbudget.Limits{Active: 240 * time.Second, Wait: 600 * time.Second, Wall: 840 * time.Second, MaxWaits: 1})
	defer cancel()
	if _, err := e.opts.Revalidate(ctx); err != nil {
		return nil, err
	}
	if e.opts.VerifyRelease != nil {
		if err := e.opts.VerifyRelease(ctx); err != nil {
			return nil, err
		}
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	b := &bridge{ctx: ctx, token: hex.EncodeToString(nonce), sessionID: sessionID, messageID: messageID, runID: uuid.NewString(), opts: e.opts, binding: e.binding, history: conversation}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server := &http.Server{Handler: b, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go server.Serve(listener)
	defer server.Close()
	stop := context.AfterFunc(ctx, func() { server.Close() })
	defer stop()
	base := "http://" + listener.Addr().String()
	// No provider credentials leave the host; the capability expires with this task.
	tweaks := map[string]any{}
	toolJSON, _ := json.Marshal(e.opts.Tools)
	for _, n := range e.binding.Nodes {
		fields := map[string]any{"bridge_url": base, "task_capability": b.token, "node_key": n.ID}
		if n.Kind == "mcp" {
			fields["tools_json"] = "base64:" + base64.StdEncoding.EncodeToString(toolJSON)
		}
		tweaks[n.ID] = fields
	}
	if err := b.emit(ctx, event.EventAgentThought, "start", event.AgentThoughtData{Content: "执行已配置流程，使用当前员工授权资源", Done: true, RunID: b.runID, NodeKey: "langflow", DefinitionVersion: bindingVersion(e.binding)}); err != nil {
		return nil, err
	}
	raw, err := e.callFlow(ctx, sessionID, query, tweaks)
	// ServeHTTP handlers have completed before inspecting the run accumulator.
	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil {
		if b.failure != nil {
			_ = b.emit(ctx, event.EventAgentThought, "failure", event.AgentThoughtData{Content: "Langflow 任务未完成：" + b.failure.Error(), Done: true})
			return &b.state, fmt.Errorf("Langflow 任务未完成：%w", b.failure)
		}
		return &b.state, err
	}
	if b.failure != nil {
		return &b.state, fmt.Errorf("Langflow调用的企业能力未全部完成，未发布结果：%w", b.failure)
	}
	answer, err := flowAnswer(raw)
	if err != nil {
		return &b.state, err
	}
	scope, err := e.opts.Revalidate(ctx)
	if err != nil {
		return &b.state, err
	}
	refs := b.refs
	if len(refs) > 0 {
		refs, err = e.opts.FilterEvidence(ctx, scope, refs)
		if err != nil || !publicationEvidenceMatches(b.refs, refs, e.binding.ReviewedExperts) {
			return &b.state, fmt.Errorf("知识权限已变化，未发布结果")
		}
	}
	if e.opts.BeforePublish != nil {
		if err := e.opts.BeforePublish(ctx); err != nil {
			return &b.state, err
		}
	}
	if e.opts.VerifyRelease != nil {
		if err := e.opts.VerifyRelease(ctx); err != nil {
			return &b.state, err
		}
	}
	if e.steer != nil {
		pending, _, err := e.steer.PollSteer(ctx, sessionID, messageID, 0)
		if err != nil {
			return &b.state, err
		}
		if len(pending) > 0 {
			return &b.state, fmt.Errorf("本轮验证暂不支持运行中追加指令")
		}
	}
	if err := b.emit(ctx, event.EventAgentReferences, "references", event.AgentReferencesData{References: refs}); err != nil {
		return &b.state, err
	}
	if err := b.emit(ctx, event.EventAgentFinalAnswer, "answer", event.AgentFinalAnswerData{Content: answer, Done: true}); err != nil {
		return &b.state, err
	}
	refsAny := make([]any, len(refs))
	for i, ref := range refs {
		refsAny[i] = ref
	}
	if err := b.emit(ctx, event.EventAgentComplete, "complete", event.AgentCompleteData{SessionID: sessionID, MessageID: messageID, FinalAnswer: answer, TotalSteps: len(b.state.RoundSteps), AgentSteps: b.state.RoundSteps, KnowledgeRefs: refsAny, Usage: &b.state.TurnUsage, Extra: map[string]any{"execution_engine": "langflow", "flow_id": e.binding.FlowID, "release_id": e.binding.ReleaseID, "run_id": b.runID, "model_calls": b.modelCalls, "tool_calls": b.toolCalls}}); err != nil {
		return &b.state, err
	}
	b.state.FinalAnswer = answer
	b.state.KnowledgeRefs = refs
	b.state.IsComplete = true
	return &b.state, nil
}

func publicationEvidenceMatches(original, current []*types.SearchResult, reviewed bool) bool {
	if reviewed {
		return reflect.DeepEqual(original, current)
	}
	return len(original) == len(current)
}

func (e *Engine) callFlow(ctx context.Context, sessionID, query string, tweaks map[string]any) ([]byte, error) {
	if e.opts.ServiceAPIKey != "" {
		client, err := langflowapi.New(e.opts.FlowURL, e.opts.ServiceAPIKey)
		if err != nil {
			return nil, err
		}
		flow, err := client.GetFlow(ctx, e.binding.FlowID)
		if err != nil {
			return nil, err
		}
		digest, err := langflowrelease.Digest(flow.Data)
		if err != nil || digest != e.binding.GraphDigest || flow.UserID != e.binding.RuntimeOwnerID {
			return nil, fmt.Errorf("发布工件已变化，拒绝执行")
		}
		if err = disableNativeMemory(flow.Data, tweaks); err != nil {
			return nil, err
		}
		body, _ := json.Marshal(map[string]any{"input_value": query, "input_type": "chat", "output_type": "chat", "session_id": sessionID, "tweaks": tweaks})
		return client.Run(ctx, e.binding.FlowID, body)
	}
	// Fixed loopback endpoint: no user-controlled URL, redirect or remote auto-login.
	if !e.opts.LocalAutoLogin {
		return nil, fmt.Errorf("本阶段需要显式开启本机测试登录，正式服务认证尚未接入")
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, e.opts.FlowURL+"/api/v1/auto_login", nil)
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("本机Langflow尚未就绪")
	}
	io.Copy(io.Discard, io.LimitReader(res.Body, 64*1024))
	res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("本机Langflow测试会话未就绪")
	}
	body, _ := json.Marshal(map[string]any{"input_value": query, "input_type": "chat", "output_type": "chat", "session_id": sessionID, "tweaks": tweaks})
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, e.opts.FlowURL+"/api/v1/run/session/"+e.binding.FlowID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Langflow任务未完成：%w", ctxError(ctx, err))
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 || len(raw) > 4*1024*1024 {
		return nil, fmt.Errorf("Langflow任务返回异常（HTTP %d）", res.StatusCode)
	}
	return raw, nil
}
func ctxError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("本机执行服务连接中断")
}
func flowAnswer(raw []byte) (string, error) {
	var response struct {
		Outputs []struct {
			Outputs []struct {
				Results struct {
					Message struct {
						Text string `json:"text"`
					} `json:"message"`
				} `json:"results"`
			} `json:"outputs"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", err
	}
	for _, group := range response.Outputs {
		for _, output := range group.Outputs {
			if s := strings.TrimSpace(output.Results.Message.Text); s != "" {
				return s, nil
			}
		}
	}
	return "", fmt.Errorf("Langflow未返回最终消息")
}
