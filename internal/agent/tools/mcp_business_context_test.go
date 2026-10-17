package tools

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/event"
	internalmcp "github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	sdkserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

func TestBusinessInquiryBindsActualSession(t *testing.T) {
	tool := newTestMCPTool("crm", "crm-service", "ask_business")
	tool.mcpTool.InputSchema = []byte(`{"type":"object","properties":{"question":{"type":"string"},"thread_id":{"type":"string"}}}`)
	for _, claimed := range []string{"", "another-session"} {
		input := MCPInput{"question": "哪些客户减少最多？", "thread_id": claimed,
			"conversation_context": map[string]any{"lastPlanId": "P-previous"}}
		bound := tool.bindBusinessSession(input, &ToolExecContext{SessionID: "actual-session"})
		require.Equal(t, "actual-session", bound["thread_id"])
		require.Equal(t, "P-previous", bound["conversation_context"].(map[string]any)["lastPlanId"])
		require.Equal(t, claimed, input["thread_id"], "do not mutate model arguments")
	}
}

func TestSalesMechanismBindsActualSession(t *testing.T) {
	tool := newTestMCPTool("crm", "crm-service", "analyze_sales_mechanism")
	tool.mcpTool.InputSchema = []byte(`{"properties":{"thread_id":{"type":"string"}}}`)
	input := MCPInput{"product_name": "柴油", "thread_id": "model-invented"}
	bound := tool.bindBusinessSession(input, &ToolExecContext{SessionID: "actual-session"})
	require.Equal(t, "actual-session", bound["thread_id"])
	require.Equal(t, "model-invented", input["thread_id"])
}

func TestBusinessInquiryActualHTTPCallKeepsHostSessionAfterApproval(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	server := sdkserver.NewMCPServer("business-test", "1", sdkserver.WithToolCapabilities(false))
	var received map[string]any
	server.AddTool(sdkmcp.NewTool("ask_business", sdkmcp.WithString("question", sdkmcp.Required()), sdkmcp.WithString("thread_id")),
		func(_ context.Context, request sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			received = request.GetArguments()
			return sdkmcp.NewToolResultText(`{"ok":true}`), nil
		})
	httpServer := httptest.NewServer(sdkserver.NewStreamableHTTPServer(server, sdkserver.WithStateLess(true)))
	defer httpServer.Close()
	manager := internalmcp.NewMCPManager(nil)
	defer manager.Shutdown()
	service := &types.MCPService{ID: "crm", Name: "CRM", TenantID: 7, Enabled: true,
		URL: &httpServer.URL, TransportType: types.MCPTransportHTTPStreamable}
	tool := NewMCPTool(service, &types.MCPTool{Name: "ask_business",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"question":{"type":"string"},"thread_id":{"type":"string"}}}`)}, manager, &proxyApprovalGate{}, 0)
	ctx := WithToolExecContext(catalogTestContext(), &ToolExecContext{SessionID: "actual-session",
		EventBus: event.NewEventBus(), ApprovalCtx: catalogTestContext()})
	result, err := tool.Execute(ctx, json.RawMessage(`{"question":"哪些客户减少最多？","thread_id":"model-invented"}`))
	require.NoError(t, err)
	require.True(t, result.Success, result.Error)
	require.Equal(t, "actual-session", received["thread_id"])
	require.Equal(t, "哪些客户减少最多？", received["question"])
}

func TestBusinessInquiryDoesNotBindWithoutHostSession(t *testing.T) {
	tool := newTestMCPTool("crm", "crm-service", "ask_business")
	tool.mcpTool.InputSchema = []byte(`{"properties":{"thread_id":{}}}`)
	input := MCPInput{"question": "销售额", "thread_id": "external-thread"}
	for _, meta := range []*ToolExecContext{nil, {SessionID: ""}} {
		require.Equal(t, input, tool.bindBusinessSession(input, meta))
	}
}

func TestBusinessInquiryBindingDoesNotChangeOtherContracts(t *testing.T) {
	for _, tc := range []struct{ name, schema string }{
		{"get_customer", `{"properties":{"thread_id":{}}}`},
		{"ask_business", `{"properties":{"question":{}}}`},
		{"ask_business", `invalid`},
	} {
		tool := newTestMCPTool("other-service", "other-id", tc.name)
		tool.mcpTool.InputSchema = []byte(tc.schema)
		input := MCPInput{"question": "销售额"}
		require.Equal(t, input, tool.bindBusinessSession(input, &ToolExecContext{SessionID: "actual-session"}))
	}
}
