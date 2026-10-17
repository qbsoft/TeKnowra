package einoruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"strings"
)

func BuildBusinessMultiAgent(ctx context.Context) (compose.Runnable[string, string], error) {
	g := compose.NewGraph[string, string]()
	if err := g.AddLambdaNode("delegation", compose.InvokableLambda(delegateBusiness)); err != nil {
		return nil, err
	}
	if err := g.AddEdge(compose.START, "delegation"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("delegation", compose.END); err != nil {
		return nil, err
	}
	return g.Compile(ctx, compose.WithGraphName("BusinessReviewMultiAgent"))
}

type expertCapability struct {
	name, query string
	r           *run
}

func (t *expertCapability) Info(context.Context) (*schema.ToolInfo, error) {
	name := "execute_contract"
	if t.name == "credit_expert" {
		name = "execute_credit"
	}
	return &schema.ToolInfo{Name: name, Desc: "执行平台授权的原始用户任务。无需参数，不允许更改知识范围、用户或客户。", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})}, nil
}
func (t *expertCapability) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	var input map[string]json.RawMessage
	if json.Unmarshal([]byte(args), &input) != nil || input == nil || len(input) != 0 {
		return "", fmt.Errorf("expert execution takes no resource parameters")
	}
	if _, done := t.r.expertResults[t.name]; done {
		return "", fmt.Errorf("expert already executed in this task")
	}
	t.r.expertResults[t.name] = "" // mark before execution; errors cannot be retried silently
	old := t.r.nodePrefix
	t.r.nodePrefix = t.name
	defer func() { t.r.nodePrefix = old }()
	var graph compose.Runnable[string, string]
	var err error
	if t.name == "contract_expert" {
		graph, err = BuildContractReviewV2(ctx)
	} else {
		graph, err = BuildCustomerCredit(ctx)
	}
	if err != nil {
		return "", err
	}
	answer, err := graph.Invoke(ctx, t.query)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(answer) == "" {
		return "", fmt.Errorf("expert result missing")
	}
	t.r.expertResults[t.name] = answer
	// No CRM fields or retrieved source content enter the planner model.
	return `{"completed":true,"result_stored_by_platform":true}`, nil
}
func delegateBusiness(ctx context.Context, query string) (string, error) {
	r, ok := ctx.Value(runKey{}).(*run)
	if !ok {
		return "", fmt.Errorf("multiagent context missing")
	}
	r.expertResults = map[string]string{}
	var delegates []tool.BaseTool
	for _, spec := range [][2]string{{"contract_expert", "合同审查专家，负责合同金额、付款安排、制度审批和合同风险；不查询CRM。"}, {"credit_expert", "客户信用专家，负责本人CRM权限内的客户信用档案、额度和评级；不审查知识库合同。"}} {
		expert, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{Name: spec[0], Description: spec[1], Model: &taskChatModel{run: r, agent: spec[0]}, MaxIterations: 3, Instruction: "必须调用唯一执行工具完成任务一次。工具已绑定原始用户任务，你不能选择资源或更改客户。工具成功后只回复‘专家任务已完成’，不编写业务数据或结论。", ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: []tool.BaseTool{&expertCapability{name: spec[0], query: query, r: r}}, ExecuteSequentially: true}}})
		if err != nil {
			return "", err
		}
		delegates = append(delegates, adk.NewAgentTool(ctx, expert, adk.WithAgentInputSchema(schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}))))
	}
	main, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{Name: "business_supervisor", Description: "企业只读业务任务协调", Model: &taskChatModel{run: r, agent: "business_supervisor"}, MaxIterations: 5, Instruction: "根据用户问题自主选择全部相关专家。仅合同问题只调用contract_expert，仅客户信用问题只调用credit_expert，综合问题调用两位专家。每位专家最多调用一次，工具参数为{}。不得执行其他操作，不能根据专家完成标记编造业务结论。所有所需专家完成后只回复任务协调已完成，平台会展示保存的专家结果。", ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: delegates, ExecuteSequentially: true}, EmitInternalEvents: true}})
	if err != nil {
		return "", err
	}
	_, err = stage(ctx, "supervisor", "主 Agent：自主选择业务专家", func(r *run) error {
		it := adk.NewRunner(ctx, adk.RunnerConfig{Agent: main, EnableStreaming: false}).Run(ctx, []*schema.Message{schema.UserMessage(query)})
		mainCompleted := false
		for {
			ev, ok := it.Next()
			if !ok {
				break
			}
			if ev.Err != nil {
				return fmt.Errorf("multiagent delegation failed: %w", ev.Err)
			}
			if ev.Output == nil || ev.Output.MessageOutput == nil {
				continue
			}
			msg, e := ev.Output.MessageOutput.GetMessage()
			if e != nil {
				return e
			}
			if msg != nil && ev.AgentName == "business_supervisor" && msg.Role == schema.Assistant && len(msg.ToolCalls) == 0 && strings.TrimSpace(msg.Content) != "" {
				mainCompleted = true
			}
		}
		if !mainCompleted || len(r.expertResults) == 0 {
			return fmt.Errorf("主 Agent 未完成真实专家委派，不能发布结果")
		}
		for _, result := range r.expertResults {
			if result == "" {
				return fmt.Errorf("专家任务未完成，不能发布结果")
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	var answer strings.Builder
	_, err = stage(ctx, "aggregate", "主 Agent：汇总已核验专家结果", func(r *run) error {
		for _, name := range []string{"contract_expert", "credit_expert"} {
			if text := r.expertResults[name]; text != "" {
				label := "合同专家"
				if name == "credit_expert" {
					label = "信用专家"
				}
				fmt.Fprintf(&answer, "## %s\n\n%s\n\n", label, text)
			}
		}
		answer.WriteString("以上为只读专家结果；未下单、付款或回写。")
		return nil
	})
	return answer.String(), err
}
