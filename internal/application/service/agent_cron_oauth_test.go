package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
)

// 定时任务以创建人的身份、用创建人存下的令牌去调按人授权的 MCP 工具。创建人还没授权、或者
// 令牌满 30 天到期之后，没有任何人在场能点「去授权」——交互式对话里的那套「弹卡片等人点」
// 在这里只会让任务干等到超时（默认 600 秒）再失败，而且失败记录里看不出是授权的问题。

func oauthNotice(bus *event.EventBus, serviceID, serviceName string) {
	_ = bus.Emit(context.Background(), event.Event{
		Type: event.EventMCPOAuthRequired,
		Data: event.MCPOAuthRequiredData{ServiceID: serviceID, ServiceName: serviceName},
	})
}

func newOAuthTestExecutor(sessions *fakeSessions, messages *fakeMessages) *agentExecutor {
	return &agentExecutor{
		sessions: sessions,
		messages: messages,
		agents:   &fakeAgents{agent: &types.CustomAgent{ID: "agent-1"}},
		repo:     &fakeCronRepo{},
	}
}

// 这条是「不干等」的来源：上游的 MCP 工具看到这个标记就不会挂起等人授权，而是立刻发一条通知后继续。
func TestAgentExec_RunsNonInteractiveForMCPOAuth(t *testing.T) {
	var marked bool
	sessions := &fakeSessions{qa: func(ctx context.Context, _ *types.QARequest, _ *event.EventBus) error {
		marked = types.IsMCPOAuthNonInteractive(ctx)
		return nil
	}}
	if _, err := newOAuthTestExecutor(sessions, &fakeMessages{}).Execute(context.Background(), agentJob()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !marked {
		t.Fatal("定时任务的上下文没有标成「无人可授权」，遇到没授权的 MCP 服务会干等到超时")
	}
}

func TestAgentExec_MissingAuthorizationFailsWithActionableReason(t *testing.T) {
	sessions := &fakeSessions{qa: func(_ context.Context, _ *types.QARequest, bus *event.EventBus) error {
		oauthNotice(bus, "svc-crm", "CRM-MCP")
		emit(bus, "抱歉，我暂时无法访问经营数据。")
		return nil
	}}
	messages := &fakeMessages{}
	out, err := newOAuthTestExecutor(sessions, messages).Execute(context.Background(), agentJob())
	if err == nil {
		t.Fatal("工具因为没授权而用不了，这次执行不能记成成功——否则任务看上去一直正常、其实一直没取到数")
	}
	for _, want := range []string{"CRM-MCP", "授权"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("失败原因 %q 里应当包含 %q，让人一眼知道该去做什么", err.Error(), want)
		}
	}
	if out != "抱歉，我暂时无法访问经营数据。" {
		t.Errorf("模型说的话要照样带回去，got %q", out)
	}

	// 任务的会话里也要看得到原因，不能只有模型那句含糊的「无法访问」。
	var closing string
	for _, m := range messages.updated {
		closing = m.Content
	}
	if !strings.Contains(closing, "CRM-MCP") || !strings.Contains(closing, "抱歉，我暂时无法访问经营数据。") {
		t.Errorf("会话里的收尾消息应同时有模型的回答和授权原因，got %q", closing)
	}
}

func TestAgentExec_SeveralUnauthorizedServicesAreAllNamedOnce(t *testing.T) {
	sessions := &fakeSessions{qa: func(_ context.Context, _ *types.QARequest, bus *event.EventBus) error {
		oauthNotice(bus, "svc-crm", "CRM-MCP")
		oauthNotice(bus, "svc-crm", "CRM-MCP") // 同一个服务的两个工具各发一次
		oauthNotice(bus, "svc-srm", "SRM-MCP")
		return nil
	}}
	_, err := newOAuthTestExecutor(sessions, &fakeMessages{}).Execute(context.Background(), agentJob())
	if err == nil {
		t.Fatal("expected failure")
	}
	if strings.Count(err.Error(), "CRM-MCP") != 1 || !strings.Contains(err.Error(), "SRM-MCP") {
		t.Errorf("每个服务点名一次，got %q", err.Error())
	}
}

// 运行本身已经报错时，保留原来的错误——授权提示是补充，不能盖掉真正的故障。
func TestAgentExec_RunErrorIsNotMaskedByOAuthNotice(t *testing.T) {
	sessions := &fakeSessions{qa: func(_ context.Context, _ *types.QARequest, bus *event.EventBus) error {
		oauthNotice(bus, "svc-crm", "CRM-MCP")
		return context.DeadlineExceeded
	}}
	_, err := newOAuthTestExecutor(sessions, &fakeMessages{}).Execute(context.Background(), agentJob())
	if err == nil || !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("原始错误丢了：%v", err)
	}
	if !strings.Contains(err.Error(), "CRM-MCP") {
		t.Errorf("授权原因也应当带上：%v", err)
	}
}
