# Eino 多 Agent 产品接入实施计划

> 执行方式：用户在审阅设计后回复“开干吧”，本会话按 executing-plans 直接执行，不重复请求授权。

Goal：发布 business-review-multiagent-v1，正式聊天中自主委派合同及信用专家。
Architecture：ADK 主、子 Agent 使用平台模型；专家调用既有业务节点。混合权限能力由平台装配，专家顺序运行、共享身份与预算，平台组合审核后的结果。
Tech Stack：Go、Eino v0.9.21 ADK/compose、现有 chat.Chat、Vue 聊天事件。
Spec：../specs/2026-10-11-eino-multiagent-product-design.md

## 约束与裁决

- 本机实施，不提交、推送或上线；保留已有未提交修改与绑定。
- 当前目录依赖上一阶段未提交实现，复用当前 checkout；不重置工作区。
- 只读、单任务、顺序委派。专家各执行业务一次。模型调用总上限16；旧定义上限4不变。
- 共享当前活动120秒、OAuth等待600秒、总720秒预算，不能子任务重新分配预算。
- 独立模型适配不移植研究流式 goroutine；使用现有 ReadModel 汇总完整响应，再提供 ADK 消费接口。
- CRM 字段不进入主/子 Agent 的工具返回；最终答复由已验证专家文本组合，专家工具仅返回完成标记。

## Task 1：混合能力

Files：einodef/catalog.go、service/eino_definition_config.go、agent_service_eino.go、agent_service_eino_credit.go、agent_service_eino_test.go。

- [x] 写混合检索与固定本人OAuth工具配置测试，确认当前拒绝。
- [x] 发布新定义；提取 MCP 能力工厂，知识与 MCP 可组合。原 MCP-only 身份复核仍保留。
- [x] 测试正确配置通过，额外工具、错服务、未明确知识、技能/记忆仍拒绝；原定义回归。

## Task 2：ADK 模型与委派

Files：einoruntime/adk_model.go、multiagent.go、multiagent_test.go、engine.go、publication.go。
Interfaces：BuildBusinessMultiAgent(context.Context) (compose.Runnable[string,string],error)；ADK model.ToolCallingChatModel；run 中累计调用、专家结果与阶段前缀。

- [x] 使用工具调用假模型写三个路径测试：合同、信用、双专家；当前 unsupported 定义失败。
- [x] 文本模型/工具参数/推理元数据适配，ReadModel 检查结束信号，共享调用与 usage。
- [x] ADK AgentTool 专家只执行既有业务图，使用原用户问题。专家事件标识与引用/业务结果保存在同一 run；禁止重复业务执行。
- [x] 主 Agent 仅选择专家，工具响应不携带业务结果；平台组合专家文本，最终重核知识和信用访问。
- [x] 测试失败不完成、取消、未知或重复专家、无专家伪造答案、调用上限和引用；记录实际输出。

## Task 3：本机产品验收

- [x] 针对性服务/运行时/配置/OAuth测试与编译；需要的前端变更仅限步骤与能力展示。
- [x] 产品界面创建独立 Agent、明确知识范围与 MCP 工具；服务端添加新绑定，使用 dev-app.ps1 重启并核实运行构建。
- [x] 单合同、单信用、综合任务正式聊天；检查专家选择、实际调用、引用及持久化 run_id。
- [x] OAuth 等待和同任务恢复；如需本人登录交接用户，继续独立验收。
- [x] 原三定义必要回归；保存截图、测试输出、限制与结果报告，不宣称全量测试通过。

## Review Focus

- 模型伪造专家执行：无实际业务结果不能成功。
- 改写客户：业务节点必须使用原用户输入，雪花ID仍字符串。
- 旧引用在信用单任务中误要求知识：没有使用的知识不输出伪引用；最终复核不赋新权。
- 同任务重复阶段覆盖：专家node_key有前缀，统一递增iteration和唯一事件ID。
- 授权等待失去身份或预算：沿用当前context/OAuthgate，不创建独立任务预算。

## 执行记录

2026-10-11：用户批准设计并要求开工，开始 Task 1/2 的失败测试。

2026-10-11：Task 1–3 完成。真实单合同、单信用、双专家、子专家授权暂停及同一 run 恢复通过；原三定义正式聊天回归通过。针对性测试、编译、运行构建哈希核验通过。独立审查发现空引用 nil/empty P2，已先复现再修复并复核。结果和局限见 D:/TeKnowra/research/selection/eino-delivery-cost-20261011/多Agent产品接入验收.md。全量测试/生产上线未完成，不列为本轮范围。
