# TeKnowra Agent 执行边界：收缩后的第一步

日期：2026-09-24。状态：代码调查与设计稿；未实施、未测试运行行为。

后续修正：双方源码静态推演没有证明本稿建议的“先整理执行边界”为必要前提，该前置建议已撤回。以 `D:/TeKnowra/research/TeKnowra-DeerFlow源码静态推演-2026-09-24.md` 的证据与结论为准；本稿仅保留历史讨论，不作为实施依据。

本稿取代《2026-09-24-deerflow-integration-design.md》作为近期工作建议；原稿仅保留为未来远程执行方案参考，不代表已批准实施。

## 目标

长期目标是企业级 Agent 基础设施：保留 TeKnowra 的企业平台能力，能够增强或替换执行核心。当前步骤只建立可维护的执行边界，不立刻融合两个完整平台，也不宣称结构整理本身提高 Agent 能力。

## 代码事实

| 已有边界 | 实际位置 | 结论 |
|---|---|---|
| AgentEngine 接口 | internal/types/interfaces/agent.go | 已有 Execute、SetMemoryPrompt、SetSteerSink，不需要再从零定义同名抽象 |
| 执行器工厂 | internal/application/service/agent_service.go:183 | 接收 Go chat.Chat / rerank.Reranker、event bus；明确是进程内工厂 |
| 会话执行流程 | internal/application/service/session_agent_qa.go | 调用工厂前后负责模型、历史、沙箱、附件、记忆、steering 等，存在本地内核耦合 |
| 范围解析 | session_agent_qa.go 的 buildAgentConfig | 知识范围、共享只读、MCP/Skill 选择与沙箱技能装配混合在同一流程 |
| 工具装配 | agent_service.go 的 registerTools / registerMCPTools | 可复用现有知识/MCP能力，但不是已经存在的跨进程授权工具服务 |
| 事件与持久化 | internal/handler/session/agent_stream_handler.go | 处理工具、引用、审批、OAuth、完成和消息保存；复用价值高，也需保持载荷契约 |
| 取消 | internal/handler/session/qa.go | 当前以取消 context 驱动进程内停止，不等价于远程任务取消 |
| 另一工厂调用方 | internal/application/service/tenant_skill_install.go:884 | 技能安装也创建 AgentEngine，不能只测聊天路径 |

## 关键结论

1. **可以分离，但已有接口不等于远程引擎已经可插拔。** Go 对象、回调、上下文与 AgentState 都不能直接当作跨进程协议。
2. **最小切入点在 session AgentQA 的运行准备与执行衔接处。** 只改 CreateAgentEngine 会保留上层本地沙箱等强制步骤；全面重写 handler 又会牵动 SSE、存储和停止等成熟路径。
3. **第一步应保留原内核的行为和工厂。** 先分清平台职责与本地执行准备，避免对尚未接入的第二个内核设计大量空接口。
4. **增强原内核与接入 DeerFlow 都是后续选项。** 前者减少跨进程成本但需要维护推理/编排能力；后者复用执行能力但新增身份、状态与工具桥成本。当前证据不足以断言哪一种总成本必然更低。

## 建议的最小改动

### A. 平台准备

保留现有登录与请求授权、会话和智能体访问检查。提取明确的“运行输入准备”步骤，集中得到：调用者身份、资源授权范围、请求输入、已选智能体及已有消息标识。

输入属于后端内部数据结构，不让浏览器提交已授权身份，也不把 Go context 整体序列化。保留调用者空间和资源来源空间的区别。

不要重新发明一套权限算法。沿用现有 resolveKnowledgeBases、SearchTargets、共享只读和 MCP/Skill 选择逻辑；只在行为已被测试覆盖的地方移动职责。

### B. 原生执行准备与运行

把本地模型实例化、模型窗口和图片能力、rerank、LoadAgentHistory、沙箱 turn 持有、附件 staging、记忆注入、steer sink 和执行组装放入明确的原生执行路径。

特别注意 buildAgentConfig 当前会调用 skillsForRun，不能把它整体贴上“引擎无关”标签。首步先标明平台范围解析与本地资源装配各自的输入输出，按依赖拆开，保持调用顺序和资源释放语义。

继续调用已有 CreateAgentEngine 和 AgentEngine.Execute，返回现有错误并产生现有事件。不要同时更换模型配置、prompt、工具名称或状态存储。

### C. 事件出口

沿用 event bus 与 AgentStreamHandler。第一步不创建第二套 run 表、消息表或事件总线，不重做前端，不开放执行后端选择按钮。

待第二个内核接入时，以实际差异决定是否增加运行生命周期接口；远端取消、状态查询和恢复不能用空实现伪装成支持。

## 第一阶段明确不包括

- DeerFlow 部署或连接，远程知识工具桥与能力令牌。
- 新的用户/空间系统、数据库 schema 或凭据转存。
- 新的子 Agent 调度、持久后台任务或 Skill 自演进。
- CRM 工具、审批、OAuth 行为变化。
- 前端改版、切换现有会话内核或生产上线。

这些取舍把第一步限定为已有流程的结构整理，但相关路径属于高敏感共享代码，不能按普通机械重命名处理。

## 三种路线的成本判断

| 路线 | 新增复杂性 | 主要维护责任 | 当前建议 |
|---|---|---|---|
| 直接完整接入 DeerFlow Gateway | 高：双任务状态、授权桥、事件、文件 | 两个上游接口与部署 | 推迟 |
| 直接补强原生 Agent | 视具体能力而定；授权/工具链复用较多 | 自己维护规划、委派、恢复与评测 | 保留候选，不能视为天然省事 |
| 先整理原生运行边界 | 相对较低，无新分布式系统 | 小范围 fork 差异与回归 | 推荐第一步 |

不在未实现和实测前给出人日或可靠性承诺。第一步降低后续选择的耦合成本，不降低跨系统授权本身的固有难度。

## 验证要求

现有测试可作为基础：session_agent_qa_scope_test.go、agent_share_runtime_test.go、session_attachment_staging_test.go、steer_live_run_test.go 及相关原生 Agent 测试。本轮只检查测试存在，未运行，不声称通过。

实施时优先验证以下行为：

1. 相同输入得到相同的知识/MCP/Skill 范围，共享智能体不扩权。
2. 原模型、rerank 和图片处理配置保持一致。
3. 附件 staging 在工具执行前完成，沙箱 turn 无论成功、报错、取消都释放。
4. 记忆只注入一次，历史不重复装载或写入。
5. 工具事件、引用、用量与完成事件仍能被现有 handler 保存，终态不重复。
6. 取消与 steer 不丢失、不重复执行；技能安装的另一调用路径不回归。
7. 最后浏览器验证普通问答、知识引用、附件、工具、停止和历史查看。

只新增针对拆分边界的行为测试，不为每个新 helper 写镜像测试。发现依赖扩散到权限语义或持久化 schema 时应停止扩大改动，重新评估边界。

## 后续决策门槛

第一步通过后，选择一条具体业务任务作为第二个执行实现的验证对象，不同时接两套外部平台。

若选择 DeerFlow：复用其编排/上下文能力，企业工具继续由 TeKnowra 执行；身份、取消和知识范围需要真实端到端验证。之前完整设计稿只供此时取用。

若选择增强原生内核：围绕真实失败任务选择一项可验收能力，例如有界子任务委派及交付检查；不能仅添加“计划工具”或修改提示词就宣称达到企业级 Agent 基础设施。

无论选择哪条，判断指标都是任务完成质量、人工干预、授权正确性、失败恢复、耗时和费用，而不是框架名称或功能列表长度。
