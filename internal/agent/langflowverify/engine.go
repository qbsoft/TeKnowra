package langflowverify

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/einoruntime"
	"github.com/Tencent/WeKnora/internal/agent/runbudget"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

const (
	// Only this dedicated local verification Agent can opt into the spike.
	AgentID       = "eb6720cc-70f1-4eab-af63-b65e61b96d84"
	FlowID        = "442b5cb9-64fc-4acc-b679-ba71bebc6142"
	ModelNode     = "TeKnowraModel-verify"
	KnowledgeNode = "TeKnowraKnowledge-verify"
	CRMNode       = "TeKnowraCRM-verify"
)

func Enabled(agentID string) bool {
	return agentID == AgentID && os.Getenv("TEKNOWRA_LANGFLOW_VERIFY") == "loopback-spike-v1"
}

type Engine struct {
	opts  einoruntime.Options
	steer types.SteerSink
}

var _ interfaces.AgentEngine = (*Engine)(nil)

func New(opts einoruntime.Options) (*Engine, error) {
	if opts.Model == nil || opts.Events == nil || opts.Revalidate == nil || opts.Retrieve == nil || opts.FilterEvidence == nil || opts.CallReadOnly == nil || opts.ReadOnlyServiceID == "" {
		return nil, fmt.Errorf("Langflow验证需要完整的企业能力回调")
	}
	return &Engine{opts: opts}, nil
}
func (e *Engine) SetMemoryPrompt(string)                               {}
func (e *Engine) SetSteerSink(s types.SteerSink)                       { e.steer = s }
func (e *Engine) SetContextCheckpointSink(types.ContextCheckpointSink) {}

func (e *Engine) Execute(parent context.Context, sessionID, messageID, query string, history []chat.Message, images ...[]string) (*types.AgentState, error) {
	if len(history) > 0 {
		return nil, fmt.Errorf("Langflow临时验证请使用新会话")
	}
	for _, v := range images {
		if len(v) > 0 {
			return nil, fmt.Errorf("Langflow临时验证仅接受文本")
		}
	}
	ctx, cancel := runbudget.New(parent, runbudget.Limits{Active: 240 * time.Second, Wait: 600 * time.Second, Wall: 840 * time.Second, MaxWaits: 1})
	defer cancel()
	if _, err := e.opts.Revalidate(ctx); err != nil {
		return nil, err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	b := &bridge{ctx: ctx, token: hex.EncodeToString(nonce), sessionID: sessionID, messageID: messageID, runID: uuid.NewString(), opts: e.opts}
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
	tweaks := map[string]any{
		ModelNode:     map[string]any{"bridge_url": base, "task_capability": b.token},
		KnowledgeNode: map[string]any{"bridge_url": base, "task_capability": b.token},
		CRMNode:       map[string]any{"bridge_url": base, "task_capability": b.token},
	}
	if err := b.emit(ctx, event.EventAgentThought, "start", event.AgentThoughtData{Content: "Langflow 可视流程开始；使用当前员工的知识与本人 CRM 权限", Done: true, RunID: b.runID, NodeKey: "langflow", DefinitionVersion: "langflow-verify-v1"}); err != nil {
		return nil, err
	}
	raw, err := callFlow(ctx, sessionID, query, tweaks)
	// ServeHTTP handlers have completed before inspecting the run accumulator.
	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil {
		if b.failure != nil {
			_ = b.emit(ctx, event.EventAgentThought, "failure", event.AgentThoughtData{Content: "Langflow 原型任务未完成：" + b.failure.Error(), Done: true})
			return &b.state, fmt.Errorf("Langflow 原型任务未完成：%w", b.failure)
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
	if len(b.refs) == 0 || len(b.customers) == 0 {
		return &b.state, fmt.Errorf("合同知识或真实客户信用未实际查询，不能通过验收")
	}
	scope, err := e.opts.Revalidate(ctx)
	if err != nil {
		return &b.state, err
	}
	refs, err := e.opts.FilterEvidence(ctx, scope, b.refs)
	if err != nil || len(refs) != len(b.refs) {
		return &b.state, fmt.Errorf("知识权限已变化，未发布结果")
	}
	for id := range b.customers {
		data, err := b.invoke(ctx, "check_customer_access", map[string]any{"customer_id": id})
		if err != nil || data["customer_id"] != id || data["readable"] != true {
			return &b.state, fmt.Errorf("客户权限已变化，未发布结果")
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
	if err := b.emit(ctx, event.EventAgentComplete, "complete", event.AgentCompleteData{SessionID: sessionID, MessageID: messageID, FinalAnswer: answer, TotalSteps: len(b.state.RoundSteps), AgentSteps: b.state.RoundSteps, KnowledgeRefs: refsAny, Usage: &b.state.TurnUsage, Extra: map[string]any{"execution_engine": "langflow", "flow_id": FlowID, "run_id": b.runID, "model_calls": b.modelCalls, "tool_calls": b.toolCalls}}); err != nil {
		return &b.state, err
	}
	b.state.FinalAnswer = answer
	b.state.KnowledgeRefs = refs
	b.state.IsComplete = true
	return &b.state, nil
}

func callFlow(ctx context.Context, sessionID, query string, tweaks map[string]any) ([]byte, error) {
	// Fixed loopback endpoint: no user-controlled URL, redirect or remote auto-login.
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:7861/api/v1/auto_login", nil)
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
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:7861/api/v1/run/session/"+FlowID, bytes.NewReader(body))
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
