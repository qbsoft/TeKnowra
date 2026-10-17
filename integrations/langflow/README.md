# TeKnowra Langflow 研发接入（A+B+C）

本轮提供独立执行器、授权资源目录和通用模型/知识/MCP组件。流程在 Langflow 画布中配置，执行由 TeKnowra 已登录员工发起；既有 Eino 定义和执行器保留。固定版本发布与普通服务身份认证已接入，当前仅支持本机回环拓扑，不等同生产部署。

2026-10-15新增成组交付工具见 `delivery/README.md`：白名单版本包、安装/原子激活/回退、解释器与依赖核验、隔离启动器、由活动清单生成共享网络Compose配置。原生隔离真实运行及Docker网络探针通过；本轮没有构建或验收完整Linux平台容器栈，不能将native-validation包作为生产容器包。证据见 `D:/TeKnowra/research/selection/langflow-delivery-test-20261015/delivery-evidence.json`。

## 页面创建和发布

2026-10-16新增通用页面入口：创建智能体时选「Langflow 可视编排」，沿用模型、指定知识库、选定 MCP 工具与技能表单；创建后在编辑窗口创建通用流程，或关联当前空间有编辑权限的已登记流程的独立副本。绑定由后端按已保存 Agent 和现有权限生成并原子持久化到 release store 的 drafts，无须逐 Agent 手改绑定清单。旧静态绑定仍兼容，不自动迁移历史。

点击「打开编排」调整画布，再同步资源、固定发布；未接入或未发布不能执行。新建会初始化 Agent Instructions，后续以冻结画布指令为准。同步校验授权范围，不自动增删画布节点。关联不接受任意外部 Flow ID，防止统一编辑服务身份跨空间误绑定。可选计算脚本策略由研发填写固定入口与输入 schema。

页面说明与本机验收见 `docs/superpowers/langflow-page-entry-validation-2026-10-16.md`。这个增量未进入封存 M/N；封包需包含 `templates/agent-components.json` 及新版后端，并纳入发布元数据备份。本机候选使用独立新元数据文件，旧程序不能读取含 drafts 的新格式。

## 旧手动接入方式

以下步骤用于保留的静态绑定兼容路径。新建智能体优先使用上面的页面入口；初始化服务身份和版本存储仍由管理员一次性完成。

1. 将 `LANGFLOW_COMPONENTS_PATH` 指向本目录下的 `components`，将本目录加入 Python 的 `PYTHONPATH`。组件复用现有 Langflow 的 lfx、httpx、jsonschema、pydantic，不复制模型供应商密钥。
2. 在 Langflow 创建流程，添加 TeKnowra 模型、知识检索或 MCP 工具组件，连接原生 Agent 节点。提示词、专家节点与连线在原生画布中配置。
3. 在服务端绑定清单声明 Agent ID、工作空间、Flow ID、资源允许范围和宿主组件节点 ID。一个节点的 kind 为 `model`、`knowledge` 或 `mcp`。清单只限制能力，不授予员工新权限；运行资源还必须满足平台当前 Agent 配置和员工权限。
4. 本机通过 `dev-app.ps1 -LangflowBindingsFile <清单路径>` 启用。这会开启显式本机测试模式；不要与旧 `-LangflowVerification` 同时启用，也不要向环境文件写票据。
5. 在 TeKnowra 编辑对应 Agent，点击“复制目录连接”，粘贴到组件的“连接 TeKnowra 资源目录”。刷新后会出现名称与稳定 ID；目录连接字段自动清空。模型参数、知识库、MCP 服务和工具在表单中选择；工具留空表示该服务的全部已绑定工具。
6. 保存图后重载确认配置，再从 TeKnowra 点击“在聊天中试运行”。仅从 Langflow Playground 直接运行宿主组件会因没有任务能力失败，这是预期行为。

绑定形状（占位值不可直接运行）：

```json
{"bindings":[{"agent_id":"<Agent UUID>","flow_id":"<Flow UUID>","workspace_id":10000,"models":["<模型 UUID>"],"knowledge_bases":["<知识库 UUID>"],"tools":[{"service_id":"<服务 UUID>","name":"<工具名>"}],"nodes":[{"id":"<模型节点 ID>","kind":"model"},{"id":"<知识节点 ID>","kind":"knowledge"},{"id":"<MCP 节点 ID>","kind":"mcp"}]}]}
```

纯知识场景可省略 tools 和 MCP 节点；纯工具场景可省略 knowledge_bases 和知识节点。CRM 信用工具可声明 `guard: "crm-credit"`，同时绑定 `check_customer_access`；此业务策略在宿主中复核，不写入通用执行器。

## 授权与数据边界

- 目录连接有效期15分钟，只读资源元数据，不含模型密钥、服务地址、Header 或 OAuth令牌。连接只在 Langflow 进程内按研发用户和平台 Agent 保存；进程重启或到期必须重新连接。
- 每次聊天生成独立回环地址和任务能力，绑定原员工、空间、会话及消息，执行结束关闭。保存图不包含目录票据或任务能力。
- 运行中重新检查员工状态、成员资格、角色、Agent 资源选择、模型、知识范围和工具元数据；撤权或配置不一致时终止。本人 MCP OAuth、参数校验、审批及授权卡片沿用宿主机制。
- MCP 保留 text 和 structuredContent，客户等大整数 ID 应为字符串。未支持的图片、音频等返回明确拒绝。
- 当前目录服务和执行器只支持本机回环部署。旧显式验收模式可以使用自动登录；固定发布模式禁止自动登录，使用独立编辑/运行 API key。正式网络、SSO 和多进程部署需要后续设计。

## 固定版本发布与最小认证

1. 使用 Langflow 官方普通账号创建一个编辑身份和一个运行身份。绑定清单中的 Flow 必须属于编辑身份；两个身份都不能是超级管理员。关闭 `LANGFLOW_AUTO_LOGIN` 和自助注册。`bootstrap_local_release.py` 仅用于旧本机测试环境迁移：保留原流程，新建独立私有草稿，清除复制图中的临时能力和全局变量引用。凭据只存本机受限文件，禁止进 Git、说明文件或图。
2. 服务凭据文件仅有 `editor_api_key`、`runtime_api_key`、`runtime_owner_id` 三个字段。启动宿主时同时传入 `-LangflowBindingsFile`、`-LangflowReleasesFile`、`-LangflowCredentialsFile` 和 `-LangflowLibraryRoot`；不得搭配旧 `-LangflowVerification`。本机库目录必须与实际 Langflow 安装一致。
3. 在 TeKnowra 对应 Agent 编辑页填写版本名称，点击“发布当前草稿”。宿主重新检查编辑权限与资源，校验已批准组件、源码、工具描述和组件依赖；用运行身份导入新的私有 Flow，读回校验后持久保存版本并激活。未发布时禁止聊天试运行。
4. 后续编辑草稿不会改变已发布工件。新任务在开始时取得活跃版本副本，运行前后检查工件/依赖摘要，步骤和完成信息携带实际 release_id。模型必须由 TeKnowra 模型节点连入每个 Agent；本阶段不允许原生供应商入口、动态附件、全局变量或外部子流程。
5. 选择已发布版本并点击“激活所选版本”进行回退，仍检查当前员工权限和依赖。图、支持模块、Langflow/lfx、LangChain/LangGraph 及其独立运行包、模型 SDK 或 MCP 工具描述变化会拒绝执行，需要重新发布；回退不等同自动回退 Python 安装版本。

运行 Flow 不开放编辑入口，普通编辑账号不能修改运行账号的私有工件。管理员与 OS 管理员属于可信运维边界，禁止在线修改运行工件；源码摘要不是恶意 Python 沙箱，也不是远端镜像证明。发布库为本机单进程文件，超过16 MiB或1000个版本时拒绝写入并保留旧库。导入成功后持久化失败可能留下未激活的私有 Flow；不自动删除其他数据。

## 本阶段限制

固定发布模式执行独立私有工件，旧显式验收模式仍执行草稿。不支持运行后重启恢复、长期记忆和图片输入。共享 Agent 接入复用平台原有组织共享关系；代码已接入，真实账号验收状态见本轮记录。桥调用串行，模型输出缓冲后转发。目录不是业务权限沙箱，每个专家共同受同一次任务的资源上限约束。以上限制不能据本轮接入宣称全部企业 Agent 底座完成。

## 文本多轮

固定发布认证模式支持同一TeKnowra会话的文本追问。历史只来自宿主会话数据库，不使用Langflow本地记忆；每轮在已验证图的全部原生Agent上注入 `n_messages=0`，输入/输出节点不保存本地消息。模型请求按“当前系统指令→宿主历史→当轮任务/工具消息”的顺序组合，不累计全局上下文或把历史写回图。

宿主为Langflow构造独立纯文本历史投影，采用原始用户文本与规范最终答案，不展开旧附件/图片描述，不回放工具调用、结果、推理和压缩checkpoint。按宿主TurnID保留同一轮最初问题与追加要求，最多最近20个完整问答、64KiB；若最新一轮本身超限，明确拒绝而非悄悄忘记。其他引擎原有历史处理保持不变。

智能推理模式本来就由TeKnowra强制开启多轮，当前没有关闭开关；协议层传空历史时仍关闭Langflow本地记忆。新会话不继承其他会话历史；切换发布版本后仍从同一宿主会话获取历史，新一轮使用当前活跃版本。历史中的业务答案仅是会话上下文，不是新授权或实时事实证明；涉及最新知识/CRM事实必须重新调用当前授权工具。

关闭记忆读取不等于禁止Langflow全部追踪/消息落库，实机仍有框架执行记录。正式部署需要配置这些数据的访问与保留策略；本阶段没有自动删除任何数据。

测试位于 `tests/test_components.py`；测试需将本目录及 `components` 均加入 `PYTHONPATH`。后端协议、目录与授权测试分别位于 `internal/agent/langflowruntime`、`langflowcatalog` 和 `internal/application/service`。

## 平台技能接入

技能包继续在TeKnowra「工具箱→技能管理」登记、安装、启停；Agent编辑页选择运行沙箱与指定技能。Langflow不保存技能包或个人环境变量，不代替现有安装器。

服务绑定增加 `skills`（技能名称列表）、`sandbox_config_id`、`allow_skill_scripts`（默认false），并将技能节点ID以 `kind: "skill"` 加入 `nodes`。只有已绑定、Agent当前选择且当前镜像可用的技能进入研发目录。技能包SHA、正文摘要和执行镜像纳入固定发布依赖；技能更新需重新发布，禁用或取消选择立即拒绝后续调用。

在画布添加「TeKnowra 技能」，粘贴新目录连接并等待刷新清空，选择平台技能，将技能工具输出连入Agent的Tools。需要脚本时开启「提供技能脚本工具」，服务绑定也必须允许脚本。发布前不得保存临时连接、任务能力、全局变量引用或供应商密钥。

读取工具默认读取SKILL.md，使用 `file_path` 读取本技能内参考资源；脚本工具接受 `script_path`、字面字符串args及stdin，入口仅为本技能scripts目录内.py/.js/.sh文件。实际执行复用宿主SkillManager、ReadFileTool、ShellExecTool和当前人员变量解析，不能将技能声明当作额外知识/MCP授权。输出与错误形成宿主工具步骤。

当前限固定发布、显式远程沙箱配置（本机Docker已验收），不覆盖Lite本机技能。脚本作者与原生沙箱仍属于既有信任边界；入口校验不构成逐技能文件系统隔离。本轮验收stdout业务结果，未验收文件产物下载。资源读取支持 `offset`（从1开始）、`limit`（最多2000行）和 `max_bytes`（最多65536字节），按返回的 `next_offset` 续读；仍保留原生单行和资源上限。工具别名在Go/Python两端统一为来源摘要加ASCII替换后截断的名称。原有Agent页面的requires_tools缺口提示识别的是原生工具清单，尚不能完整识别Langflow图里的能力桥，不据该提示扩大权限。

### 会话附件与技能组合

`allow_session_attachments: true` 可与授权知识库、只读技能组合，仍跳过全会话原始文件导出及reconcile，不移除旧文件。合同正文只经宿主校验后进入本轮文本上下文，不自动进入知识库。知识库应绑定企业制度，技能说明应区分合同事实与制度来源。

附件模式若启用 `allow_skill_scripts`，必须同时配置服务端 `structured_scripts`。每项含 `skill`、`path`、`input_schema`，随版本冻结保存；执行前匹配固定技能/入口，拒绝所有args，只接受至多64KiB的单个JSON对象。schema顶层必须为object，关闭additionalProperties；字段仅支持string/number/integer/boolean及required、enum、长度、pattern、minimum/maximum等简单约束，不接受引用或远程schema。非附件旧发布不设置该字段时沿用旧协议。

```json
{"structured_scripts":[{"skill":"tyer-contract-session-review","path":"scripts/calculate.py","input_schema":{"type":"object","properties":{"amount":{"type":"string","pattern":"^[0-9]{1,12}(\\.[0-9]{1,2})?$"},"prepayment_percent":{"type":"number","minimum":0,"maximum":100},"review_threshold_percent":{"type":"number","minimum":0,"maximum":100},"rule_reference":{"type":"string","minLength":1,"maxLength":512}},"required":["amount","prepayment_percent","review_threshold_percent","rule_reference"],"additionalProperties":false}}]}
```

这仅适用于研发审核过、已固定包SHA/镜像版本、只消费结构化输入的可信计算脚本。规则阈值与出处来自授权检索，计算脚本不验证其语义正确性。固定入口及stdin约束不是OS隔离；不可信代码、需要自行读取合同文件的脚本须另做任务级文件隔离，不能凭本开关放行。示例技能不读文件、环境变量或网络，不提供任意shell，不借用历史合同；勿将任意脚本加入计算清单。

## 共享 Agent

共享与撤销使用原有 Agent 共享管理，不另建授权表。执行前及能力调用边界重新检查共享关系和接收空间成员资格；模型、知识与技能配置取源 Agent，当前使用者身份、个人变量和 MCP OAuth 主体保持不变。知识范围重新获取共享授权后与本次任务初始范围相交，不能因运行中扩大配置而自动增加权限。研发目录与发布权限仍仅属于源空间。

OAuth工具目录仍按当前人员保存。共享接收方目录缺失、连接配置失效或缺少固定发布所需工具时，复用原生目录发现与聊天授权等待进行同步；不会借用发布者目录。同步后仍须满足个人目录的工具可用性和固定发布摘要。授权期间撤销共享或同步后仍缺工具均拒绝执行。该追加路径的现场验收状态见本轮验收记录。

接收方“停用共享”沿用平台的隐藏偏好语义；撤销授权需要源空间删除共享关系。技能沙箱与管理员配置环境变量的信任边界同原生共享 Agent，不能将共享理解为逐技能文件系统隔离。
