# Eino 企业执行链路实施计划

> 实施方式：当前会话顺序执行，使用 executing-plans 与 test-driven-development。用户已批准设计并要求“开干”，不再重复请求实施确认。

**目标：** 修复共享 OAuth 恢复链路，交付可复用的只读 Eino 执行能力。

**架构：** 保留现有 AgentEngine、工具注册、Gate 和聊天事件。业务图通过平台适配器读取授权知识与只读工具，身份和凭据不进入模型配置。

**技术：** Go / Eino / 现有 MCPManager、EventBus / Vue 聊天。

**规格：** ../specs/2026-10-10-eino-enterprise-execution-contract-design.md

## 全局约束

- 保留全部既有未提交修改；不提交、不推送、不部署。
- 保留旧合同审查路径；既有试点定义继续可用。
- 只读、web_user、进程内运行；不启用写工具、共享服务账号替代或跨租户 Agent。
- 默认累计活动120秒、授权等待600秒、墙钟720秒；每次运行最多一次重新授权等待。
- 下游记录级访问校验未核实时，不发布 CRM 结果分支。

## 复核重点

- 授权完成时旧调用 context 已过期：实际 CallTool 必须使用新上下文。
- 等待中停止、策略撤销与迟到登录：不得执行或复活任务。
- 授权等待不能重置累计活动预算；单次等待不能无限延长。
- 客户 ID 保持字符串，工具/服务配置与模型输入不能扩大权限。
- 注册、模型和最终输出都使用固定调用者；工具结果不能作为知识引用。

## 任务1：共享 MCP 授权恢复

文件：internal/agent/tools/mcp_oauth.go、mcp_tool.go、mcp_oauth_resume_test.go。

接口：新增连接辅助函数返回 client、实际 operation context 与 cancel；连接、CallTool 和 ListTools 共用该生命周期。旧调用方不受破坏。

- [x] 用真实 httptest MCP、内存 OAuth 仓库和真实 Gate 写慢授权回归测试：等待超过 ExecTimeout 后成功，业务执行一次且参数不变。
- [x] 运行测试，确认旧实现因过期 context 失败。
- [x] 保留恢复上下文到实际业务调用结束；恢复后重查策略与停止信号。
- [x] 覆盖等待中禁用、停止后授权、403、重复恢复、发现工具的慢授权。
- [x] 定向 tests 与 race 检查。

## 任务2：累计运行预算

文件：internal/agent/runbudget/budget.go、budget_test.go；mcp_oauth.go；einoruntime/engine.go。

接口：New(ctx, Limits) 返回运行 context/close；Pause(ctx) 返回 resume/error；仅运行上下文附带的预算支持人工暂停，普通 Agent 保持既有语义。

- [x] 测试累计活动耗尽、等待暂停活动、等待上限、第二次等待拒绝、父 context 停止。
- [x] 运行红测试；实现 timer 生命周期、剩余累计额度及安全关闭。
- [x] 在授权等待边界暂停/恢复，在 Eino 运行入口使用预算。
- [x] 定向测试及 race 检查，覆盖任务1慢授权。

## 任务3：定义与业务工具能力

文件：config/eino_runtime.go、service/agent_service_eino.go、einoruntime 业务定义与测试；必要时 tools 只读适配器。

接口：服务端注册稳定定义版本；工具能力使用既有 MCPTool，不把 token 暴露给业务图。

- [x] 核实 CRM 客户查询和信用读取的逐记录权限校验；记录支持与缺口。
- [x] 红测试固定定义、只读工具名单、无授权、身份一致、ID字符串、结果输出边界。
- [x] 最小实现只读节点；旧试点默认保持知识模式。客户绑定不明确不查询信用。
- [x] 交付客户信用查询样板，复用公共能力；无法满足下游契约的分支保持关闭并明确提示。
- [x] 全部相关 Go 测试与必要前端检查。

## 任务4：真实聊天联合验收

文件：本计划进度记录与研究验收文档；不保存凭据。

- [ ] 本机构建与原生启动，浏览器验收 A1–A12。
- [ ] 真实账号、慢授权、撤权、停止和引用/刷新闭环；缺少真实证据的项目明确标记待验收。
- [x] 独立代码复核，修复重要发现并回归。
- [ ] 汇报交付、实际证据、剩余限制；不把本机通过称为生产上线。

## 本机联合验收进展 2026年10月10日

已交付合同审查与客户信用两个只读定义，共用 TeKnowra 工具、OAuth、身份与活动预算。合同绑定客户后的联合信用分支保持关闭；当前信用样板只接受用户原文中的唯一完整客户名称，客户 ID 始终由后端查询并校验。

| 验收项 | 当前证据和判断 |
|---|---|
| A1 审查与引用 | 本轮真实 DeepSeek 审查、刷新、引用来源通过；来源链接在系统外部打开，通过其已展示的同一详情URL另开可控页签，已读取完整合成原文并截图 |
| A2 双账号 | 同一真实 MCPManager 并发 admin/sales01 身份正确，同客户返回总数1/0，记录权限 true/拒绝；sales01本地合同Agent正式聊天已通过；信用Agent的sales01本人CRM正式聊天正反例、ID/admin绕过拒绝及刷新回放已在2026-10-11完成 |
| A3 授权恢复 | 正式聊天撤销授权后出现卡片，恢复同 run、同工具ID且仅一份结果；过期、缓存撤权由真实 Gate/MCP httptest 集成覆盖 |
| A4 超过调用窗口 | 本轮新正式会话在实际60秒配置下等待100700ms后完成真实登录，同run成功且无重复业务调用，刷新仍在；此前600秒等待到期的失败样本单独保留 |
| A5 停止和迟到授权 | 停止后完成真实登录，仍只有一次失败客户检索，无信用读取及成功结果；重复恢复集成测试拒绝。最新版本另测停止与刷新，截图及只读DOM正文确认失败可见，AX快照遗漏该段不能作为正文为空的证据 |
| A6 知识撤权 | 当前正式聊天读取后停用合成资料，在validate拒绝且刷新无成功意见/引用，资料已恢复；此前真实Eino暂停点配合正常UI撤销共享的探针通过。已在2026-10-11以真实sales01正式聊天补测：读取后撤销合成viewer共享，在validate拒绝且刷新0引用；撤销动作是本机SQL故障注入 |
| A7 客户权限 | 新 CRM 轻量权限接口、MCP严格适配与最后输出复核已接入；真实两账号正反例和受控撤权、ID/身份/租户拒绝通过。真实客户角色运行中撤权待补测 |
| A8 故障处理 | 18个受控OAuth/审批/权限场景通过，401刷新有界、403不触发登录；真实缩写客户名被拒绝，不能猜测客户。未对真实授权服务器注入全部故障 |
| A9 复用 | 信用3节点与审查4节点各有真实聊天结果，公共OAuth/预算实现未复制 |
| A10 兼容发布 | 旧合同审查真实无业务工具聊天成功；定义配置、名单及旧Agent保护测试通过；未执行旧版审查回写 |
| A11 记录 | 成功/停止记录保留run、版本、节点、工具ID、活动/等待耗时，抽检无token字段；进程硬中断恢复保存失败和节点身份，未保证所有未落盘工具细节永久保留 |
| A12 刷新中断 | 活跃授权卡片刷新可操作；实际杀掉本机执行进程后原会话曾无终态，已补修并真实回归：明确中断、停止生成状态结束，再次刷新失败文字仍存在。首期不续跑 |

浏览器证据位于 `D:/TeKnowra/research/selection/eino-next-20261009/`，会话中断样本 `5dc3e5f9-3483-4bf6-8c3e-3c8b1e047bd4`，慢授权样本 `09094c49-235b-455d-a279-90942102f88b`，停止后授权样本 `6794bb6c-7ffd-4e77-9baf-a738d187219d`，新合同样本 `5af9a5bb-3f13-4510-a636-d8ed4d8c2fb1`，旧版样本 `70603124-c53e-41b5-a8cc-032586a5b7e3`。

最新停止回放样本 `41406a52-f0b5-4773-a6d3-eb0ed1e7790b`，截图 `credit-eino-stop-refresh-final.jpg`；中断截图 `credit-eino-process-interrupted.jpg`，原文截图 `contract-eino-source-fulltext.jpg`。实际60秒窗口补测会话 `9b103961-21bd-436a-8ece-8289a07bf233`，外部登录由用户完成，尚未收到本次补测登录结果。

该补测最终等待600000ms，仅一次失败的find_customer（活动31ms），没有信用读取或成功结果。CRM-MCP超时已通过正常UI恢复180秒并重新打开核对；本轮为补测撤销的admin CRM授权仍为未授权，恢复需正常登录，不回填已撤销令牌。超时样本初次显示通用context canceled，随后新增运行取消原因回归，保留授权等待预算超时与用户主动停止的不同原因；修改前的历史消息不伪装为修改后结果。

追加修复按RED→GREEN完成：信用定义失败回放、失去进程主体的终态处理、信用错误的前端显示。后端handler定向/race通过，前端聊天定向28项通过，Vite构建首次默认堆内存不足，使用本次进程6144MB上限重跑通过，未修改全局Node配置。

最终本机后端通过 `dev-app.ps1` 启动，PID9516，前端代理身份配置success=true。追加授权等待原因修复后，einoruntime/runbudget完整race通过，最新浏览器会话 `481af66d-c5d1-450c-be1f-409a73b366e3` 主动停止并刷新后正文明确显示“任务未完成：任务已停止：context canceled”，截图 `credit-eino-stop-cause-final.jpg`。没有把修复前600秒超时样本的显示当成修复后实测。

这份记录不把受控测试、独立真实服务探针和完整浏览器闭环混为一类。当前是本机只读交付验收，尚不满足全部 A1–A12 联合验收完成的声明条件；不提交、不推送、不部署生产。

## 追加真实验收 2026年10月10日 23:38–23:46

### A4：实际调用窗口过期后的授权恢复通过

通过正常 MCP 配置界面将 CRM-MCP 超时从180秒设为60秒，确认当时 admin 为未授权。新建信用 Agent 正式聊天 `1b4ee311-cb70-4cde-82d0-7af338e12a1a`，等待卡片显示剩余8:39后点击授权（已等待超过80秒），用户在本次外部授权页完成登录。

原会话成功返回3节点、4次工具调用，刷新后结果仍在。持久记录：run `345e2d2f-9162-4653-8edf-8a18677f7690`，墙钟110257ms；find_customer工具ID `389a9492-c31e-4995-8812-2a403bfe1f81`，等待100700ms、活动4848ms；后续权限检查、信用读取、输出前权限检查各一次，等待均0。没有重复客户检索或信用读取。此次等待确实超过实际60秒窗口，与此前180秒窗口样本区分。

截图：`credit-eino-strict60-wait-new.jpg`、`credit-eino-strict60-resumed.jpg`、`credit-eino-strict60-resumed-refresh.jpg`。随后正常UI恢复180秒，再打开确认180秒与已授权；临时配置页已关闭。没有把旧的已超时任务复活。

### A6：当前版本正式聊天的来源停用拦截通过，用户撤权仍单独记录

本地临时观察程序只读观察新会话的Redis节点事件；进入analyse（已完成retrieve）后，仅将合成文档 `a2c85c42-6394-4df7-b96c-5d3f3c076928`、tenant10000的enable_status临时改为disabled。会话 `e1a1bc64-28e2-4a8f-955c-77f5801c8131`，注入时刻 `2026-10-10T15:43:01.5617181Z`。实际DeepSeek执行进入validate后拒绝，正文明确显示审查未完成、knowledge source no longer valid，4节点，无成功审查意见或引用；刷新后失败仍可见。

观察窗口结束时defer恢复原enabled状态，更新1行；另一个只读数据库探针确认synthetic_source_status=enabled。该程序没有停止执行进程，没有改其他文档或分块。截图 `contract-eino-source-fault-current.jpg`、`contract-eino-source-fault-current-refresh.jpg`。这是资料可用性故障注入，不计作真实用户或组织授权撤销。

### A2/A7：真实身份与正式发布边界

本轮查实际tenant_members：admin仅在10000为active owner，sales01仅在10001为active owner；两者不是同工作空间成员。当前信用定义禁止SharedAgentReadOnly，并要求执行租户等于真实CallerTenant、MCP服务属于该租户。补充配置回归覆盖SharedAgentReadOnly=true必须拒绝，定向测试通过。因此，跨工作空间消费新信用Agent不是换个登录即可完成的当前范围验收；需要合法发布到销售工作空间及其本人OAuth服务，或单独设计跨工作空间服务委托，不能伪造CallerTenant绕过。

再次使用真实OAuth仓库和同一个MCPManager并发查询：selftest分别为admin、sales01；同一客户admin total1、readable=true，sales01 total0且check_customer_access业务拒绝。该探针仍使用服务所属工作空间作为服务上下文，只证明MCP按人令牌及下游数据范围隔离，不替代新信用Agent的sales01正式聊天。

CRM代码复核：assertReadable每次经getScoped/applyScope读取当前客户归属；管理员数据范围不限，单纯转移客户不能让admin失去读取权限。真实运行中客户撤权必须使用有受限数据范围的用户与专用合成客户。已请求用户提供可临时变更归属并恢复的专用测试客户，未更改现有业务客户或角色。A7的真实运行中客户撤权仍未验收；受控输出复核测试与实际静态双账号正反例不能替代它。

本轮没有修改产品执行逻辑；只增加一项跨工作空间配置拒绝回归及验收记录，未提交、未推送、未部署。

## 2026年10月11日：销售工作空间正式验收准备与身份异常

- 实机确认 admin 属于10000、sales01属于10001；两个已发布Eino定义在 session_agent_qa 入口都拒绝 SharedAgentReadOnly。此前仅说明信用定义的表述不完整。销售工作空间以本地实例复用固定定义，知识通过原有只读共享获取，禁止伪造 CallerTenant。
- 用户明确批准现有 DeepSeek 对所有工作空间作为内置模型可用。仅将模型36d34fea-e7f4-44d1-83eb-3d09c3e05401的is_builtin设为true；没有读取或复制模型凭据。正常sales01界面已能创建Agent。
- 正常界面创建销售本地合同Agent 8cd0fcd8-b41d-46e6-ae65-200641706d18，以及独立OAuth MCP服务4f95e0dd-aa99-4628-a3d2-ac4c51fa7871。信用Agent副本c2ebdfb6-fa58-4e71-8fba-7aa3076b7025仍在编辑，尚未接入Eino或验收。
- 本轮新增合成知识库3631c0e3-0f4d-4882-945b-5d430773b570到测试销售组织de97174d-a914-429d-b1e4-cb272ce7f855的viewer共享；同时创建的原合同V2 Agent测试共享不能直接执行Eino，需验收后撤销这份新增测试共享，不涉及既有经营问数共享。
- 同一真实MCPManager探针已改为各自真实home tenant和各自服务。首次销售服务授权后，selftest证实平台sales01的CRM授权实际账号是admin，客户检索和访问也随admin权限；立即通过正常界面撤销。不能计为sales01权限隔离通过。
- 原因边界：平台回调验证OAuth state/原主体并按主体存token，但没有校验上游OAuth账号与平台账号是否为同一人。MCP通过CRM /user/info解析实际username，故上述账号错配有真实服务证据。它不是共享服务账号降级，也不是没有admin凭据即可提权；但严格本人一致约束仍有缺口，应另行设计issuer+subject映射，不盲目把通用OAuth username字符串相等当作产品契约。
- 用户随后回复已授权，但2026-10-11本轮重查销售服务token行数为0，正常UI未授权，工具刷新失败；未获得可确认的sales01新grant。已再次发起正常授权并请用户使用sales01完成。后续探针添加身份不符即停止业务查询的保护。
- 本段仅记录本机设置与事实，不修改产品身份逻辑、不提交推送、不部署生产。A2销售正式聊天、A6正式共享撤权、A7真实运行中客户撤权仍待后续证据。
后续现场门槛：销售合同实例已追加到既有ignored .env.local映射，dev-app.ps1原生重新构建并启动PID36968，/health返回status=ok。正常sales01新聊天选择该本地Agent时，界面拒绝就绪并提示缺少重排模型；模型投影确认qwen3-rerank 4cac7f1b-6854-4763-ba60-3c85af4379f6只属于10000且非内置。因此未发送合同问题、未产生Eino成功/失败运行，不能以“创建成功”代替正式聊天通过。已询问用户是否允许也将该重排模型设为内置，未擅自扩展模型共享范围。

销售信用草稿已切换自定义类型、不使用知识库并取消全部遗留RAG工具；本人OAuth尚未落库，所以三项CRM工具还不能通过界面加载勾选，草稿尚未保存发布或映射。服务用途说明已保存，服务正常页面仍未授权。再次点击去授权后，请求用户在此次新页使用sales01完成。

新增ignored tmp/eino_share_revoke.go只是下一轮的本机故障注入准备，尚未运行验证。它仅锁定合成知识库的单份viewer共享，以及真实sales01/10001的新正式聊天，在analyse开始后软删除该份共享并观察持久终态；计划使用SQL注入而非伪称正常admin界面操作，不删除知识库、组织成员或其他共享。

浏览器证据：sales-contract-rerank-required-20261011.png、sales-mcp-unauthed-20261011.png；相关页签已保留，信用配置草稿仍未保存。当前具体待输入为正确CRM本人授权回调及重排模型共享选择。现有A2/A6/A7不能标记完成，未修改产品代码，未提交推送或上线。
## 2026年10月11日 01:08–01:10：授权重试与销售合同正式闭环

用户明确说明本轮CRM使用admin登录，并批准将现有qwen3-rerank设为内置模型。仅更新模型4cac7f1b-6854-4763-ba60-3c85af4379f6的is_builtin；未读取或复制模型凭据。正常sales01界面已经出现内置qwen3-rerank，合同本地实例通过UI选择并保存。

CRM独立销售服务正常UI显示已授权。真实同一MCPManager自检：平台sales01使用的该服务上游client_id=admin，与用户说明一致；探针对销售主体的错配分支不执行客户业务查询，不能计作sales01本人CRM低权限验证。保留用户本轮明确选择的admin授权，未再次擅自撤销；严格本人一致约束缺口仍不能标为解决。本轮证明新的授权可保存、连接和真实自检成功，不是新的聊天OAuth暂停/恢复闭环（此前有独立正式样本）。

销售合同基线：session4ba7cbee-571d-4a49-b321-4f1b77f39cf9，持久session owner=sales01、tenant10001，run cfe7effb-944b-4849-9ce4-1e0e23f1faa3，定义contract-review-v1；plan/retrieve/analyse/validate四节点完成，5134ms，3个分块引用对应1篇合成文档。浏览器真实DeepSeek返回新政策门槛、预付款担保、违约条款缺失，未执行业务写入；刷新同结果及引用保留。截图sales-contract-baseline-refresh-20261011.png。

A6真实运行中共享撤权：新session53e85664-5674-48f9-a922-3ffdf4e57727，owner=sales01、tenant10001，run b1420698-7969-4045-88a8-f1474cb6a307。本机观察脚本在retrieve完成、analyse开始事件后，于2026-10-11 01:09:29.507（UTC+8）软删除唯一合成KB viewer共享1行；不伪称admin正常UI撤销，未停用原始文档或移除组织成员。validate返回知识授权已失效，正式界面未输出审查意见、0引用，刷新拒绝文字仍在。截图sales-contract-revoked-refresh-20261011.png。失败持久AgentDurationMs为0，不能用它声称已正确记录失败墙钟耗时；4节点、run/version和完成/失败状态已保存。

清理：独立读库确认合成源文档仍enabled，合成KB活跃测试共享0；本轮此前新增的原合同V2 Agent到测试销售组织的临时共享也定向软删除1行，没有改变既有经营问数共享、组织权限或业务数据。销售本地Agent及历史验收会话保留。两份内置模型配置按用户要求保留，因此销售合同实例再次获取该合成资料需要合法重新共享。

本轮没有产品代码修复，没有提交推送或生产部署。A6当前正式共享撤权缺口已补齐；A2/A7中sales01本人CRM和专用合成客户运行中记录撤权仍不能视为完成。
## 2026年10月11日 01:18–01:23：sales01 本人CRM低权限正式验收

用户要求本人低权限验证。销售服务通过正常UI撤销上次admin grant，再发起新的OAuth；用户明确以sales01完成授权。同一真实MCPManager、各自真实home tenant及各自独立服务的自检分别返回admin、sales01，均按人OAuth。相同客户山东佳航能源集团有限公司：admin total1/access true；sales01 total0、check_customer_access业务拒绝。没有把平台主体改成admin，也没有回填旧token或使用共用服务账号。

通过sales01正常界面将先前副本c2ebdfb6-fa58-4e71-8fba-7aa3076b7025保存为“客户信用 Eino 销售验收”，自定义、不使用知识库、指定独立销售MCP服务、关闭长期记忆/技能/网络扩展，只选三项只读CRM工具。复制留下的search_knowledge勾选在知识工具禁用时无法取消，使用本机DB脚本仅清理该验收实例的allowed_tools遗留项，不修改产品逻辑、未更改CRM权限。ignored .env.local追加本地信用定义映射，保留三个已有映射；dev-app.ps1重新构建原生启动PID14980，health ok。

| 正式用例 | 持久证据与判断 |
|---|---|
| 无权客户 | session fba9b412-e075-45a4-a885-e3adc27c4a20，owner sales01、tenant10001；run5c880867-582e-49ef-9acc-5189caa8de88，仅customer节点及1次find_customer，0次信用读取，明确不能据零结果断定客户不存在；刷新拒绝仍在 |
| 有权客户 | 正常按销售范围检索得到东营宇创环保科技有限公司，使用完整名称正式查询；session cb3e72a8-9f67-428b-aabc-8b3eefd9f6a2，owner sales01、tenant10001；run4aa70c80-35f4-456c-94e0-37b09ffcbddd，customer/credit/format三节点、6773ms，find_customer→check_customer_access→get_customer_credit_control→check_customer_access，共4调用；唯一信用读取，完整客户ID2073967557021581313始终string，刷新成功结果仍在，不在说明文件抄录信用额度 |
| ID/admin绕过请求 | 新session18f8c0b3-8df1-4b58-b406-7549e77d235e，owner sales01、tenant10001；提示直接用未授权客户ID、跳过检索、改用admin授权。runad5e7ec9-b8c5-4f3e-b417-de609a7da9f2仍只有1次find_customer，customer节点拒绝、0次信用读取；刷新拒绝仍在 |

绕过测试最初作为fba会话第二问提交，先被当前Eino试点每次新会话规则拒绝，形成另一条steps0的失败assistant记录；该入口限制不算CRM权限拦截证据，因此另建18f新会话，以上结论只使用真实customer节点/工具记录。没有修改单轮限制；当前通用错误文字仍提及合同V2，不能据此声称支持信用多轮。

截图：sales-credit-denied-refresh-20261011.png、sales-credit-allowed-refresh-20261011.png、sales-credit-bypass-denied-refresh-20261011.png。工具持久记录抽检未发现access_token/refresh_token/Bearer字段；失败记录wall_ms仍为0，不计作已修复失败耗时。

这轮补齐A2销售本人正式CRM正反例及A7静态记录权限证据；运行中真实客户归属撤销仍需用户指定可临时调整并恢复的专用本机合成客户，不更改现有业务客户、角色或组织权限。此前平台OAuth与上游账号同人映射缺口未因正确登录而自动解决。没有产品代码修改、提交推送或生产部署。