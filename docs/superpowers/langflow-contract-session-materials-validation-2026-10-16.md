# 合同会话材料与Langflow附件接入（2026-10-16，本机验证）

## 范围与数据原则

合同由用户在聊天中上传，使用现有临时附件上传、异步解析及预览链路。不自动创建合同知识文档；该合同会话的新问答也不自动进入`__chat_history__`，不提取长期记忆。企业制度仍可作为有权限的常设知识库使用。

历史消息、历史索引、原件、旧沙箱文件和封存M/N均不删除、不回填、不重建。聊天、临时原文、模型输入输出与运行trace仍可能持久保存；“不自动入库”不等于“无任何存储”。既有临时附件TTL和清理任务保持原样。

## 实现

- Agent配置`disable_chat_history_indexing`默认false。显式开启后，新助手消息的`execution_context.chat_history_indexing_disabled`写为true；后续同会话换Agent仍继承。索引入口每次按会话所属空间查持久标记，包含软删除消息；查询失败不创建知识。无数据库迁移。
- `allow_session_attachments`默认false，隐含上述禁止索引与禁止长期记忆。仅对Langflow文本投影启用；原生合同Agent只开前一个开关。
- 每轮从本会话临时文档重新读取正文，验证tenant/session、有效期、ready状态、类型、非空正文和预算，不采用旧消息里的截断正文。追问在本会话仅一份唯一文档时可自动复用；多份材料需本轮明确上传/选择目标材料。
- 沿用12000近似token附件预算，同时限制累计正文64KiB、最多5个附件。超限明确失败，不输出全文已审查。TXT、Markdown、可提取文本的PDF和Word可接入；VLM OCR因页数覆盖无法证明，首版拒绝。解析器产生的文本本身仍需业务抽检，不能把ready状态当成合同法律审查质量保证。
- 正文通过已有`MessageAttachments.BuildPrompt`的不可信材料包装进入原Chat Input。原问题及LF文本历史保持独立，不重放历史附件正文。不新增Langflow文件上传器或知识库。
- 固定发布从当前Agent配置捕获附件开关；运行及激活重新核对，旧发布不能因修改草稿开关扩大能力。
- 独立review发现原通用staging会导出全会话附件库存。新文本路径跳过整个staging/reconcile过程；首版附件模式不与技能、沙箱脚本混用，避免技能读取其他失效材料或旧文件。原有非附件Agent技能能力保持原样。

## 验证记录

初始RED：`research/selection/langflow-handoff-20261016/contract-red.log`，新字段及方法缺失。
首次定向GREEN：同目录`contract-green-final.log`，types、repository、handler、service、config通过；其中langflowdef/catalog被过滤后无匹配测试，不计入通过测试项。
独立review补充RED：`contract-review-red.log`。最终修复回归见`contract-review-final.log`。
前端：AgentEditorModal脚本/模板编译通过；langflowConnection现有两项测试通过。未声称全仓类型检查或全部测试通过。

本机真实验收已完成（admin，复用现有DeepSeek）：

| 验收 | 结果与证据 |
| --- | --- |
| TXT上传与不重传追问 | 同一会话`9240ab1f-44cc-49d8-8243-4d8aeef6f803`共4条消息；正确输出123450元、40%、尾部码7319，追问49380元。2条真实LF trace均ok。 |
| Word DOCX上传 | 会话`bc65efd6-d1fd-4516-8985-ee2a3695f4e1`，正文ready，金额、比例、编号及尾部码正确，LF trace ok。 |
| 可提取文本PDF上传 | 会话`9bed8076-3d23-4cc8-b42e-75de65744411`，独立合同246800元、25%、尾部码8421正确，未混用前一合同金额，LF trace ok。 |
| 超限与刷新提示 | 会话`092cf926-c23a-4059-8743-fe298b23c3a7`，解析后37008近似token，显示“任务未完成：会话附件超过完整审查输入上限”，刷新后保留，未启动LF trace。 |
| 不自动入库 | 以上10条合同会话消息均无knowledge_id；5条助手消息均有持久禁索引标记；同名上传合同知识文档0。 |
| 普通会话对照 | 原有`对话录入提取`Agent会话`1d88c549-1ec8-4065-80cc-015049c60509`创建1条聊天历史索引，禁索引标记false。仅验证索引，不评价该旧Agent的结构化输出质量。内置快速问答缺少模型，本轮未替它修改配置。 |
| 历史数据 | 原合同Agent关联50条知识记录，元数据指纹与改前一致：`67a6385e408986cf5e17aa0c9bbea893`。 |

4条成功LF trace均指向本机冻结运行图`7d31ef4c-4728-4f77-b47c-c37636525b02`，发布ID`b24e6b14-edb5-4c3d-99cc-c2aa3aa5e31e`，标签`会话文本附件-V1-20261016`。不是将原生执行器成功误算成LF成功。

只读事实/计数证据：`research/selection/langflow-handoff-20261016/contract-material-business-evidence.json`，SHA256 `734e1039e1dd8d66951d8d600ad125f3c1ef2853437738f4551cb43f5b741525`。同目录`capture-contract-material-evidence.py`使用只读PG事务及只读LF SQLite；`verify-contract-material-evidence.py`的TXT/Word/PDF/超限/普通对照及历史校验全部PASS。无原合同正文或凭据导出。

本轮未重新做sales01真实账号全矩阵、未实测旧DOC或Markdown、未做OCR与长合同分页业务验收；不借用旧交付包的知识库合成合同结果充数。仍未上线；新功能尚未封装为新的正式镜像或交付包。

## 本机独立验收配置

- 新Agent：`b8c79d0a-e630-4f22-8379-7f04b10b064c`（合同会话附件 Langflow 验收）。无预置合同KB、CRM及技能依赖。
- 新草稿图：`a972f0ad-e999-4896-8e5b-a974b874380d`；四节点，复用受控TeKnowra模型。
- 新本机绑定：`research/selection/langflow-handoff-20261016/contract-bindings.local.json`，不覆盖旧绑定文件。
- 新候选二进制：`repo/tmp/WeKnora-contract-materials.exe`；保留`WeKnora.exe`。只在本机运行，不构建或更新正式交付镜像。
- 候选SHA256：`5fa8be5c0f781a651ab9b2e3814a12c248c99f3e66c06b1f2aca8ca11f7d8534`。当前8080已切换本机候选进程，配置初始化复用项目`dev-app.ps1`；前端仍5173，LF仍7861。
- 测试使用合成TXT/DOCX/PDF，无真实合同或CRM写入。

## 客户启用顺序

1. 更新到含本次源码的独立新版本；本次未封装新正式运行包，不能把封存M/N当成已包含本功能。
2. 原合同Agent只开启禁止自动入库；选用Langflow文本附件的独立Agent再开启会话附件，关闭技能和沙箱脚本并保存。
3. 原有受控Langflow绑定和模型接入方式不变，保存图后发布新版本；不复用旧发布的权限快照。
4. 用户新建聊天上传合同，制度从授权KB取得；在有效期内追问同一份合同。多份/修订版明确上传目标，超限拆分并说明局部范围。
5. 核对回答、来源和禁索引结果后再做业务审查标准验收；正式上线仍需用户另行授权。
