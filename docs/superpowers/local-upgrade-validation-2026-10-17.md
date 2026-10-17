# 上游升级隔离回归与本机切换验收

## 2026-10-17 Eino草稿与复核诊断续验

按用户要求在同一最终二进制上启用已有LLM诊断，目录为受限的 `research/upstream-runtime-20261017/private-eino-audit-diagnostic/`。捕获3次相同输入的新会话（7c0687b1-9a30-418b-ad4d-a04cfe02426b、c8e894e1-a1e1-411f-8476-e32b93c272f3、0ccabb2c-cb03-45f2-b7a5-9da50c59b264）的实际草稿、检索资料、复核输入及返回JSON；3次均16节点/8工具成功，合同与信用专家完成汇总，复核均为approved=true、issues=[]。没有替换模型或修改权限/门禁/业务代码。

对其中一份完整捕获的相同草稿与资料直接复核3次均通过；加入“本合同已付款40000元，并已通过财务审批”的无依据断言后，真实DeepSeek复核返回approved=false，明确指出资料没有付款记录或审批通过记录。负向结果与具体理由保存于受限目录live-replay.json；草稿与完整复核输入输出在captures.json，完整会话交叉核验在full-validation.json。诊断文件约200KB（截图另计），不是交付包，不提交或公开。

**根因限制仍然存在：** 原失败会话99115f98-f8e4-4ae0-b856-9ef61a506d60没有保存当时的生成草稿与具体issues；事后不能恢复未记录的内容。本轮3次相同输入没有复现拒绝，因此不能确定原失败是草稿实质错误还是模型复核误判，不能把当前通过写成“原故障已修复”。未凭猜测修改产品代码或放宽事实/权限规则。当前完整多专家技术链路3次通过，历史拒绝具体原因未确认。

诊断捕获结束后已用原launch_local.py恢复普通本机运行，诊断日志关闭；普通启动入口与源码未改，本机后端PID18168（后续重新核对），仍使用最终SHA245bc7ec…、122库及禁自动迁移配置。原5173/7861/8765与历史数据保留，不打包、不上线。

## 2026-10-17 本机续验最新结论（取消交付包后）

当前不制作交付包、不上线。旧交付介质已按用户要求清理；以下历史中的封存包位置不再有效。后端已从 PID27888 经核对路径重启为 PID12836，同一最终二进制 SHA，普通入口仍禁自动迁移；前端5173、Langflow7861、MCP8765保持原服务。

本轮补验结果：

- **通过：重启后首次技能执行。** 原 PDF 会话 c26e4780-ba34-4654-9d22-97011ecc8547 新一轮读取技能、检索制度、实际执行 calculate.py 一次；同一沙箱保留，Docker exec_die 退出0；61700.00、25%、35%、末尾码正确，问答无自动知识索引。证据 `research/upstream-runtime-20261017/local-postrestart-skill.json`。
- **通过：原5173 OAuth暂停恢复机制。** 多专家会话 d6c28907-0291-48f4-823c-0f9f279c9a5a 在 find_customer 等待本人授权51221ms，回调302跳转success，同assistant恢复并执行成功；令牌按web_user归属当前admin。该业务任务随后因我给出的客户名称不唯一匹配而拒绝，不能将其算为整个多专家任务成功。证据 `local-original-callback-oauth.json`、`local-multiagent-name-mismatch.json`。
- **通过：本机 Langflow MCP执行。** 会话9582dac0-8216-4491-9b1f-65d25c2d614d 查询准确客户名“东营宇创环保科技有限公司”；四次CRM工具调用全部成功，包括前后两次check_customer_access，雪花ID按字符串保留；固定发布trace ok，额度1000000/822000/178000一致。旧流程同时进行了两次知识检索。证据 `local-langflow-mcp.json`。
- **未通过：本机 Eino多专家整体任务。** 准确客户名的新会话99115f98-f8e4-4ae0-b856-9ef61a506d60中，信用专家成功完成查找、访问检查及取数；合同专家在validate节点被事实复核拒绝，组合未完成。代码确认该错误来自approved/非空issues门禁；当前持久记录没有保存生成草稿及具体复核issues，不能断定模型误判，也不能宣称本轮多专家通过。未放宽门禁、未修Eino代码，保留失败证据 `local-multiagent-fact-audit-rejected.json`。同一Eino定义每次任务要求新会话，旧会话重试被拒是已有约束，不是此次新回归结论。

汇总 `local-postupgrade-regression-summary.json`。当前122 clean、回退库114 clean、21个既有Agent保留。**本轮明确未通过项为Eino合同事实复核，需捕获具体草稿与issues后核对依据再决定修复；不以重复尝试直到通过代替原因核实。** 新上游cron/housekeeping等仍关闭，本轮未验收启用它们。交付包安装/回滚不再属于用户要求，不列为剩余工作。

2026-10-17。本轮已完成隔离验收与本机切换，未上线、未推送、未提交。既有未提交修改、历史记录及旧程序保留。封存 M/N 正式交付集合没有重新构建或更新，不能将本轮本机候选当作新正式交付包。

## 当前本机状态

- 8080：`D:/TeKnowra/research/upstream-runtime-20261017/WeKnora-upstream-final.exe`，实际 PID 27888（重启后应重新核对）。SHA256 `245bc7ecde35c3e6625d2eaa1df92db9e10a8edf1cb488ffb16bf654330ed5e7`。
- 源码 HEAD `aac54535a32710e9913822ff1f0bd7c03937776b`，包含上游 `e07a190219a98bbebd87f6ec60701682ef0df093` 和本地未提交的集成兼容改动。以二进制 SHA 和源码清单核对，不能仅凭界面版本号断言干净 Git 构建。
- 当前业务库 `teknowra_local_upgrade_20261017`，122 clean。原业务库 `teknowra_preupgrade_restore_20261017`，114 clean，作为停写回退检查点保留。
- 5173 原开发前端继续运行；7861 原 Langflow 使用原 SQLite、发布和服务身份恢复运行；8080/5173/7861/MCP OAuth metadata 均实测 200。
- MCP 8765 原进程 PID 30764 和原 `.env` 摘要未变，回调仍为原 `http://localhost:5173/api/v1/mcp-oauth/callback`。没有移植隔离 OAuth 令牌；既有过期授权仍应由本人走原授权流程。
- `.env.local` 已指向新业务库，`AUTO_MIGRATE=false`、`AUTO_RECOVER_DIRTY=false`，日志模板兼容已固化。新上游 cron、housekeeping 与 live-run sweep 保持关闭，未经本轮验收的新后台行为不自动启用。

## 隔离验收通过范围

| 范围 | 实际证据 |
|---|---|
| 114→122 迁移、重启 | 隔离库迁移；77 张实体表按预期变换核对；排除扩展视图；原字段不随意放宽 |
| Langflow 页面入口、发布、多轮 | 页面创建/资源保存/流程创建/固定发布；旧固定版本与新知识场景；三轮含后端重启；trace 和发布一致 |
| Eino 知识与权限 | 授权检索成功，无知识权限拒绝且刷新提示持久 |
| MCP 本人授权 | sales01 实际 CRM 身份；Eino 和 Langflow 暂停、授权后同 assistant/run 恢复；等待分别约 159/169 秒；无共用管理员降级 |
| 技能、脚本与恢复 | 独立 mTLS Docker-in-Docker；原镜像 ID 相同；脚本实际退出 0；后端重启后旧隔离沙箱三小时前活动标记被续活，哨兵保留，脚本只执行一次 |
| 合同首次上传 | TXT 和 Word 首轮成功；解析完成晚于用户消息，验证命中异步等待；金额/编号/末尾码完整；制度、技能与脚本组合成功；不自动入知识库 |
| 最终 Eino 多专家 | 最终候选会话 `37a88d27-97d2-465e-93ff-9342f0cec626`，16 节点/8 工具全部成功，合同专家与信用专家结果汇总 |
| 最终 Langflow 技能 | 最终候选 Word 会话 `c2fabfff-5d7f-46bd-8efa-18da5572d869`；同固定发布 `a60ec8ab-57e4-4a7c-9025-368d87360c1d`；一次真实计算、trace ok |

测试库 `teknowra_upgrade_20261017`、测试 MinIO/Redis、Langflow SQLite、沙箱引擎和凭据副本均独立。本轮隔离阶段原沙箱启动时间、状态与配置核对不变。测试沙箱引擎没有挂原 Docker socket；仅 loopback mTLS 端点。测试服务 18080/15173/17861、独立 MinIO/Redis 与沙箱引擎在结束时停止，卷与全部证据保留。测试中的正常闲置回收不等于脚本被打断，已归档的 exec inspection 与实际 exec_die 退出事件交叉证明。

## 本轮修复与限制

1. 上游工具保存校验误拒合法 `mcp:服务ID:工具名`：复用已有授权键解析器；发现、执行、服务配置及身份复核继续由原目录执行。有效、畸形、未知工具与撤权测试通过。
2. 新上游附件上传返回 202 后异步解析，合同入口立即拒绝：复用既有等待预算；等待后重读并复核租户、会话、过期、ready 与完整正文。取消、失败和越会话测试仍拒绝。没有用截断片段替代合同原文。
3. Eino 模型事实审查误把等价措辞和正确事实列为问题：仅调整复核提示词，明确按实质语义核对；无依据事实不要求存在相反资料。权限、引用及 approved/issues 拒绝逻辑未放宽。真实同模型正确样本通过，虚构已付款/已审批样本拒绝；最终实际多专家组合通过。

保留真实失败与质量限制：知识首答曾多加错误币种判断；事实复核模型的负向样本虽正确拒绝付款/审批，但还附带一条错误币种问题。不能声称每条模型审查理由始终准确，也不能据此放宽安全拒绝。合成材料验证的是技术链路，不是实际合同法律或企业审批效果。

沙箱网络边界须区分：隔离引擎使用 `none`；本机切换沿用原沙箱配置，未显式配置网络模式的原 Docker 沙箱仍为默认 `bridge`。本轮仅执行既有固定入口的可信纯计算脚本，不声称任意脚本无网络或具备新增 OS 安全隔离能力。

## 本机切换证据

切换前重新停写当前实际业务源，创建受限新备份：`D:/TeKnowra/research/upstream-runtime-20261017/private-cutover-backup-20261017/`。含凭据，不提交或公开。该备份包括 PG dump、原 MinIO/Redis 卷、原配置、Langflow 状态/独立 SQLite 快照及文件，77 张实体表、96 个状态文件；manifest SHA、ZIP 内容摘要及 SQLite integrity_check 全部通过。

从新备份恢复到**全新本机候选库**，恢复指纹完全匹配，再由最终程序迁移到 122；迁移后全部 77 张表核对通过，仅允许 116 默认工具、118 UTC epoch 与原环境存储别名刷新/随机加密的预期变换。没有将实验库接到 8080。随后以自动迁移关闭重启最终程序。

切换后 admin/sales01 原页面登录及历史 Agent 可读；原 21 个 Agent 保留，测试新增 CRM Langflow Agent 未进入新业务库。本机 Eino 会话 `78aa927b-d1ee-440e-af0a-b2056ddc3fa0` 授权摘要通过；Langflow PDF 会话 `c26e4780-ba34-4654-9d22-97011ecc8547` 真实读取技能及参考、检索制度、执行 calculate.py 一次，61700.00/25%/35%/false 与末尾核验码一致，exit 0、固定发布一致、trace ok，无合同问答自动索引或同名知识库文件。

## 后续启动与回退

普通本机启动（先确认原开发基础设施健康且 8080 没有其他程序监听）：

```powershell
& 'D:/TeKnowra/research/upstream-runtime-20261017/start-upstream-local.ps1'
```

该入口明确校验最终二进制 SHA，使用新业务库、原 Langflow 7861、原模型/权限/MinIO/Redis/文件/版本库；普通启动禁用自动迁移，不重新编译或接入隔离状态。

需要回退时先按实际 PID、完整可执行路径确认并停止 8080 最终程序，再执行：

```powershell
& 'D:/TeKnowra/research/upstream-runtime-20261017/start-upstream-local.ps1' -Rollback
```

回退固定使用旧 `repo/tmp/WeKnora-langflow-entry-final.exe` 和原 114 库，自动迁移关闭。它保留新 122 库，不做向下迁移，也不把切换后新增数据自动复制回旧库；回退期间继续使用 `-Rollback`，普通启动会回到新候选。不要使用历史包装启动旧程序连接新 122 库，不要重复运行一次性的 `restore_original_runtime.py`。本轮验证了迁移恢复与新程序禁自动迁移重启；未为本机切换后新增数据进行反向迁移验收。

## 工件索引

统一根目录：`D:/TeKnowra/research/upstream-runtime-20261017/`。

- `isolated-final-acceptance.json`：隔离最终门槛、最终 SHA、修复/限制。
- `eino-final-multiagent.json`、`contract-final-word-upload.json`、`isolated-skill-script-lifecycle.json`：真实执行与持久化。
- `eino-oauth-resumed.json`、`langflow-oauth-resumed.json`：此前本人授权及同任务恢复。
- `local-cutover-restored.json`、`local-cutover-migration-verification.json`、`local-cutover-smoke.json`：本机恢复、77 表迁移核对和真实冒烟。
- `local_cutover.py`、`verify_local_migration.py`、`launch_local.py`、`start-upstream-local.ps1`：受限备份、迁移核验、明确启动/回退。
- `private-backup-v2/` 为早期有效备份；v1 校验失败及误迁移的 122 库/dump 均保留，不能混用或清理。

下一步若需更新正式交付集合，须单独冻结本轮增量、更新不可变镜像/清单并按交付包范围验收；本轮没有授权上线。
