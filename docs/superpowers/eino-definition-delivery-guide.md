# 新增 Eino 业务 Agent 的交付约定

适用范围：目前支持授权知识检索、单服务按人 OAuth 只读工具，以及两者受控混合。支持已发布的顺序专家委派定义；尚不支持业务写入、多轮、图片、长期记忆，不代表所有 Agent 模式已经产品化。

## 研发交付步骤

1. 编写业务图模块：在 `internal/agent/einoruntime/` 提供 `BuildX(context.Context) (compose.Runnable[string,string], error)`；图节点使用平台传入的模型、授权检索、只读工具、预算和事件，不读取密钥，不选择执行用户。
2. 在 `internal/agent/einodef/catalog.go` 发布能力描述：唯一且版本化的定义 ID、名称、知识要求或只读工具名称列表。此目录是研发审核过的发布清单。
3. 在 `internal/agent/einoruntime/publication.go` 发布对应 Build、SystemPrompt 和必要的 BeforeOutput 业务复核。知识范围和引用的通用最终复核不能被业务钩子取代。
4. 为业务事实、错误输出、所需权限和完成状态编写测试。模型提示词不能替代代码级授权；独立模型事实复核也不能保证答案永远正确。
5. 编译后由服务端 `TEKNOWRA_EINO_AGENTS` 绑定现有 Agent ID 和定义 ID，重启本地服务，在浏览器按真实账号验收。未知定义仍拒绝，未绑定 Agent 继续原引擎；禁止重绑受保护的旧合同 Agent。

新增**现有能力档位内**的定义，不应修改配置 JSON 解析器、聊天入口能力判断、通用引擎图选择代码或 OAuth 传输实现。增加新能力档位时仍需要扩展平台接入并验收，不能承诺零基础设施改动。

## 本轮样例

`knowledge-summary-v1`：授权检索 → 摘要及合法引用 → 独立事实复核 → 通用输出前权限复核 → 引用/答案/完成事件。采用原有证据编号、预算与流式模型适配，不复用合同场景提示词。

现有合同审查和客户信用查询已经迁移到相同目录。客户信用依然使用本人 OAuth、实时服务状态检查、工具目录、参数校验、固定参数和授权卡；没有改成共用账号。

## 多 Agent 接入状态与限制

已通过产品界面新建并本机发布“授权知识摘要 Eino 验收”和“合同与信用多 Agent Eino 验收”。后者对应 business-review-multiagent-v1：ADK 主 Agent 自主选择合同/信用专家，顺序运行，各专家使用已审核业务图；共享身份、预算、事件和引用。单专家、双专家、子专家 OAuth 等待及同任务恢复均已实测。专家由研发在代码定义，尚不能将任意界面 Agent 直接组合为子 Agent；并行、动态专家、技能及重启后续跑仍未实现。生产尚未上线。

完整本机证据：D:/TeKnowra/research/selection/eino-delivery-cost-20261011/多Agent产品接入验收.md。

## 研发入口

- 授权知识场景从 [起步模板](templates/eino-knowledge/README.md) 复制，默认复用摘要拓扑；真正业务判断由研发编写，不是换名称就能交付。
- 只读 MCP 场景参考 internal/agent/einoruntime/customer_credit.go 的 Invocation、固定参数、客户匹配和 BeforeOutput。新工具名写入发布清单；其工具目录/schema/只读语义需要验收，不能因有 MCP 就视为可安全执行。
- 多 Agent 场景参考 internal/agent/einoruntime/multiagent.go：分别定义专家职责、每专家可执行能力与结果保存，协调模型只委派；不可将协调参数作为新资源授权。各专家的业务复核和通用输出复核都保留。
- IDE 生成代码先放临时目录，审阅拓扑和 Lambda 业务实现后移入；不要覆盖现有权限/业务节点。资源绑定在 TeKnowra 产品配置中完成。

MCP 工具配置使用平台编码 mcp:服务ID:工具名，必须属于所选同一服务且与发布工具集合精确匹配；知识工具为 search_knowledge（兼容旧 knowledge_search）。编码由 AgentMCPToolKey 生成，由 ParseAgentMCPToolKey 校验；不要使用会随名称或 schema 改变的运行时注册名。

公共底座提供模型、授权检索、本人OAuth、执行步骤、run_id、引用和输出复核。业务节点不直接读取 Token/Key，不私建服务账号、不重新分配任务预算。当前累计活动120秒、授权等待600秒、墙钟720秒，授权等待最多一次；模型默认最多4次、多 Agent16次。新增复杂拓扑须先估算预算，不静默放宽。

完成交付后按 [发布清单](eino-release-checklist.md) 记录证据与限制；生产部署另行安排。

每个实际业务复制 [交付记录模板](templates/eino-delivery-record.md)，分别记录业务结果、执行证据和底座改动。

完成判定不能只看数据库 is_completed：该字段表示消息已终止，失败也会设为 true。须同时核对成功完成事件、最终业务答案、工具实际成功及无错误；失败应保留“任务未完成”正文和部分执行步骤，刷新后仍可查看原因。
