# Langflow 通用接入前三项实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将本机验收原型变成由配置选择流程与资源的 Langflow 接入，完成独立执行器、动态目录、通用模型和 MCP 桥。

**Architecture:** TeKnowra 从服务端绑定清单选择运行流程，解析资源权限并建立短期任务能力；Langflow 的官方自定义组件负责资源选择与 schema 生成工具。目录会话只允许读取资源元数据，运行会话只由宿主创建并绑定当前员工，二者不混用。现有 Eino 与验收原型保留，正式接入使用独立执行器分支。

**Tech Stack:** Go、Gin、现有模型/MCP/知识服务，Python、现有 Langflow 1.12.5/lfx，现有 Vue 管理入口；不安装新框架。

**Spec:** D:/TeKnowra/research/selection/langflow-decision-test-20261013/正式接入源码审查与工作量.md，只实施 A+B+C。

## Global Constraints

- 必须用中文交流，用第一性原理思考和解决问题；先核实事实，再给结论。
- 保留已有未提交修改，不提交 `.env`、本机配置、密钥、令牌、日志及含凭据的备份。
- 不迁移 Eino、不修改生产服务、不推送、不上线、不清理全局 Docker 或持久卷。
- 三项范围不包含固定版本发布、多轮历史、技能/共享接入及正式部署；这些不得因本轮成功标记通过。
- CRM 雪花 ID 全程按字符串；MCP 按人 OAuth 不得静默降级到共用服务账号。
- Go缓存、临时文件、证据仍放D盘；不重复安装依赖，不复制已有凭据到研发组件或计划。
- 未支持的能力显式拒绝；删除合同信用断言不能删除现有 CRM 客户访问检查。

## Review Focus

- 两个组件或两个 MCP 服务同名工具不能串用服务、参数schema或员工身份：任务2/5验证复合资源键。
- 研发目录票据不能执行模型或工具；撤权后不能沿用旧目录权限：任务4验证用途、时效与重新授权。
- 配置声明的资源必须与 Agent 和当前用户允许范围取交集，不能直接变成授权：任务3验证不扩大范围。
- 通用化后纯知识流程与纯工具流程不再必须查合同/信用；缺失必要回调必须启动前报错：任务2/3验证能力组合。
- Langflow HTTP 200但工具失败、模型截断或返回空答复，不得记为整轮成功：任务2/6保留失败关闭。

## 文件与接口边界

- 新增 `internal/agent/langflowdef/`：Binding、NodeBinding、ToolBinding及严格清单解析；不持有密钥。
- 新增 `internal/agent/hostcapability/`：执行器无关的 Invocation、Options、模型读取接口；不依赖 Eino 定义。
- 新增 `internal/agent/langflowruntime/`：基于已有原型的正式独立Engine、协议桥与资源映射；不直接复制业务分支为通用实现。
- 新增 `internal/config/langflow_runtime.go`：加载服务端绑定文件与固定服务地址；不接受聊天请求选择URL或执行器。
- 新增 `internal/application/service/agent_service_langflow.go`、`agent_capabilities.go`：从已有服务准备受控能力。
- 修改 `internal/application/service/agent_service.go`、`session_agent_qa.go`及相关配置类型：新增独立分派、权限准备；保留Eino原路径。
- 新增 `internal/application/service/langflow_catalog.go`、`internal/handler/langflow.go`、`internal/router/routes_langflow.go`：受控目录会话与元数据路由，接入已有鉴权及权限矩阵。
- 新增 `repo/integrations/langflow/teknowra/` 组件包及其测试：可维护的模型、知识、MCP选择与schema工具，验收目录不作正式源码位置。
- 修改现有 Agent 管理入口：增加已绑定流程的研发目录连接及宿主试运行入口；不建设新画布。

核心接口：`langflowdef.Binding` 包含 AgentID、FlowID、WorkspaceID、节点资源类型映射、允许模型/知识库、服务及工具清单；`hostcapability.Invocation` 包含 SessionID、MessageID、RunID、NodeKey、ToolCallID、ServiceID、Tool、Args；`hostcapability.Options` 包含 `ResolveModel(ctx, modelID)`、`Revalidate(ctx)`、`Retrieve(ctx, scope, queries)`、`FilterEvidence(ctx, scope, refs)`、`CallTool(ctx, invocation)` 及 Events。`langflowruntime.New(binding, options)` 返回 `interfaces.AgentEngine`。

目录DTO只包含资源名称、稳定ID、可用状态、工具描述和input schema；不复用带连接详情的完整服务DTO。资源名称变化不影响图中保存的ID。

### 任务1：绑定清单及独立配置入口

**Files:** 新增 `internal/agent/langflowdef/binding.go`、`binding_test.go`、`internal/config/langflow_runtime.go`、`langflow_runtime_test.go`，修改 `internal/config/config.go`。

**Interfaces:** 产生 `ParseBindings(raw []byte) (map[string]Binding, error)`、`Config.LangflowBinding(agentID string) (Binding, bool)`；后续分派只读这些服务端配置。

- [ ] 先写测试：重复Agent/节点ID、未声明资源、未知字段、同Agent同时绑定Eino、含额外JSON对象时拒绝；两个不同FlowID绑定正常，不出现验收固定ID。
- [ ] 运行 `go test ./internal/agent/langflowdef ./internal/config -run 'Langflow|Bindings' -count=1`，确认新测试先失败。
- [ ] 实现严格清单解析、深复制配置与启动期校验；读取非凭据的本机绑定文件，不改已有env文件。
- [ ] 运行新测试和既有 `EinoDefinition` 测试，均通过。

### 任务2：执行器及协议桥通用化

**Files:** 新增 `internal/agent/hostcapability/`、`internal/agent/langflowruntime/` 及对应测试；旧 `langflowverify` 保留。

**Interfaces:** 消费任务1 Binding；产生 `New(binding langflowdef.Binding, opts hostcapability.Options)`。每次Execute的任务能力绑定真实上下文，注入绑定清单声明的所有宿主组件节点。

- [ ] 先写测试：纯知识、纯工具、不同模型组件、两个服务同名工具路由；未授权模型/服务、过期能力、未知请求字段拒绝。
- [ ] 运行上述包测试，确认未实现能力失败。
- [ ] 实现不依赖einodef/einoruntime的Options和执行器；移除合同/客户必查断言、BusinessMultiAgent白名单与固定节点；保留上下文、超时、参数限制及失败关闭。
- [ ] 模型通过ResolveModel按发布允许范围解析；工具使用服务ID+工具名定位。输出保留 MCP text/structured JSON，拒绝未支持的多模态，不强迫所有工具返回信用字段。
- [ ] 将CRM信用访问复核留在业务策略适配层；用既有协议测试覆盖截断、EOF及HTTP200工具失败，不把业务判断散入通用桥。
- [ ] 运行新包及旧 `langflowverify`、`einoruntime` 测试通过。

### 任务3：宿主能力准备与独立分派

**Files:** 新增 `agent_service_langflow.go`、`agent_capabilities.go`与测试；修改 `agent_service.go`、`session_agent_qa.go`及必要内部配置类型。

**Interfaces:** 为任务2 Options提供知识、模型、MCP回调；从认证上下文和已有AgentConfig产生有效范围，不从清单直接授予权限。

- [ ] 先写测试：不同能力组合、未绑定资源、跨工作空间输入、撤权后访问、服务变更后原调用拒绝；既有Eino不进入Langflow分支。
- [ ] 运行针对性service测试确认失败。
- [ ] 复用已有buildSearchTargets、来源校验、模型GetChatModel、MCP metadata与MCPTool/ApprovalGate；按需准备，不经过validateEinoDefinitionConfig。
- [ ] MCP执行保留本人Principal校验、工具schema参数校验、PinArguments与授权等待；不支持的共享/技能等配置仍明确拒绝。
- [ ] 运行新分派、Eino能力准备与权限相关针对性测试。

### 任务4：研发目录会话与调试入口

**Files:** 新增 `langflow_catalog.go`、`handler/langflow.go`、`router/routes_langflow.go`及测试；按项目容器/路由注册模式接入。

**Interfaces:** `POST /agents/:id/langflow/design-session` 由当前已登录、可编辑该Agent的研发申请短期目录票据；目录路由在票据对应身份、空间和Agent范围内返回专用DTO。票据不具备业务执行用途，不进入保存图或导出内容。

- [ ] 先写测试：未登录、无编辑权、异空间、过期或改用途票据拒绝；目录只含当前授权资源；响应无密钥、URL、Header、EnvVars、CRM令牌。
- [ ] 运行handler/service针对性测试确认失败。
- [ ] 实现随机短期目录会话，重新检查身份权限、限制目录访问用途；服务重启后会话失效并提示重新连接。
- [ ] 调试经TeKnowra创建正常文本任务，并复用同一受控执行器；不把短期目录票据当MCP令牌或模型API Key。
- [ ] 新路由依照既有路由能力矩阵检查，运行鉴权、目录及响应测试。

### 任务5：动态可视组件

**Files:** 新增 `integrations/langflow/teknowra/model.py`、`knowledge.py`、`mcp.py`、`catalog.py`、测试与安装说明；只更新独立测试环境的组件路径。

**Interfaces:** 消费任务4目录DTO，官方update_build_config刷新资源选项，运行时消费任务2注入的能力及工具描述。组件保存资源ID，表单显示名称；多个工具使用复合名称避免冲突。

- [ ] 先写Python测试：名称/ID映射、资源撤销、同名工具、string客户ID、嵌套对象/数组/enum/required schema、目录票据不进入导出图。
- [ ] 运行现有venv Python unittest，确认未实现行为失败；不新增依赖。
- [ ] 复用官方DropdownInput、update_build_config、schema工具构造与Tool Mode；动态生成模型/KB/MCP组件，移除验收UUID与固定客户函数。
- [ ] schema不兼容时准确提示，不能静默降为任意字典；组件说明写清目录授权与运行授权区别。
- [ ] Python测试通过后，在真实Langflow界面核对选择、刷新、保存、重载与工具Actions。

### 任务6：入口和整轮验收

**Files:** 修改 `frontend/src/views/agent/AgentEditorModal.vue` 及对应API/测试；新增研发接入说明与本轮验收记录。

**Interfaces:** 管理入口调用任务4会话API；试运行调用现有TeKnowra聊天，不绕过员工身份。

- [ ] 先写必要前端交互测试：未绑定Agent不展示假可用入口，目录失效提示重连；入口不把员工JWT拼进URL。
- [ ] 实现已有Agent管理中的连接目录、打开编排和宿主试运行入口；不新建流程编辑页面。
- [ ] 在真实浏览器配置模型/KB/MCP/Prompt/专家，保存重载并运行已有合同与信用图，核对本人CRM主体与来源。
- [ ] 将既有知识摘要场景配置为独立Langflow流程，以相同组件执行；其资源及图差异仅改绑定/配置，不改通用后端核心。
- [ ] 核对旧Eino行为、新runtime失败关闭及输出保存；进行整轮代码审查，修复重要问题并重跑受影响验证。
- [ ] 更新交付记录，明确A+B+C完成与D—G仍未实施；不提交、推送或生产部署。

## 实施方式与待确认项

建议由当前助手在现有工作区直接实施，保留所有历史未提交改动。普通新worktree不带已有未提交原型，且本机内存/磁盘紧张，增加另一套依赖不划算。若选择隔离worktree，只复制本阶段必要源码差异，不复制.env、凭据、数据库或测试环境；先核对路径、底层分支与工作区归属。

本计划新增的设计选择是：目录会话为短期只读用途；调试通过TeKnowra正式任务；前三项仍使用可编辑Flow绑定，不宣称已有固定版本发布。实机核实当前为普通checkout，分支 `cursor/mcp-oauth-return-to-current-page`，不是main，也不是隔离worktree。编码前需要明确沿用当前工作区还是隔离worktree。计划已自查任务依赖与接口一致性；不自动commit历史未提交修改。
