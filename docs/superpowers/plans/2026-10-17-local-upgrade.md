# 本机上游升级：备份、隔离迁移、核心回归、再切换

## 最终状态（2026-10-17 18:16，已切本机，未上线）

最终候选SHA245bc7ec…完成真实Eino多专家16节点/8工具及Word首轮技能计算，证据isolated-final-acceptance.json。此前授权恢复、权限拒绝、多轮及旧固定发布与迁移回归已通过，明确保留模型质量瑕疵和旧失败。

新停写备份当前实际业务114库（private-cutover-backup-20261017，77表、96文件）校验后恢复新库teknowra_local_upgrade_20261017，原始指纹匹配；最终binary迁移到122、77表预期变换全部通过；自动迁移关闭后重启。8080实际PID27888/最终binary，新库122 clean，原114库冻结保留，实验库没有用于8080。原5173、7861、MCP metadata健康200，MCP/CRM原进程与.env未改。

双真实账号原页面登录和历史Agent可读，本机Eino知识摘要78aa927b…成功；Langflow PDF c26e4780…读取技能/参考/制度、实际calculate.py一次、61700.00、25%未超过35%、末尾码一致、exit0、固定发布与trace一致且无合同问答入库。脚本沙箱网络配置沿用原默认bridge，未冒充隔离none策略；原配置逐JSON核对未改。证据local-cutover-smoke.json。

后续启动/回退及限制见 ../local-upgrade-validation-2026-10-17.md 和根手册最新段。测试18080/15173/17861、独立MinIO/Redis/沙箱引擎停止，所有卷/备份/候选和失败保留。未提交、推送或上线，M/N封存集合未更新。

## 技能与多专家续验（2026-10-17，最终候选待完成）

Eino多专家在mcp-compat候选完成16节点/8工具；独立mTLS Docker-in-Docker中技能report.py和calculate.py实际exit0，原沙箱状态未变。新上游附件202异步解析与AgentQA相邻，旧合同材料入口即时拒绝：已复用既有等待超时与完整正文复核，RED/GREEN通过。contract-async候选真实新上传首轮（48da9d80）成功，parse ready晚于user消息；附件/制度/技能/脚本3工具成功、49380.00/6172.50、核验码一致、无自动入库。后端重启首次技能（55ece0cd）保留原隔离沙箱和三小时前活动哨兵，真实脚本仅调用1次。证据contract-first-async-upload.json、restored-first-skill-messages.json。

新候选Eino组合复验发生模型事实audit误拒，保留失败：现有LLM_DEBUG_LOG在隔离受限目录复现，正确金额/币种/比例/日期的意见因同义阈值表达、缺失信息措辞被误拒，audit issues中混入正确点。仅修提示词要求按实质语义，无依据与相反资料分开，拒绝逻辑/权限/引用不变。真实同模型合成replay：正确等价表达通过，凭空已付款/审批拒绝；负向issues另有错误币种判断，不能声称每项issues准确。Eino测试包通过，最终binary构建中，尚未以最终binary完成组合和最终验收。独立只读review修正了无依据必须相反资料的提示问题，切换脚本审查无阻塞。

截至此处没有切换原8080。原业务114 clean、21个Agent、无最近未完成消息；测试库122 clean。MCP回到原5173白名单/.env摘要未变，原7861保持。下一步最终候选组合实际成功及完整证据后，fresh停写备份当前实际业务114，恢复新正式候选库teknowra_local_upgrade_20261017再迁移；禁止将实验库直接用于8080。全部失败、备份、旧程序和未提交源码保留，不上线。

## 最新续验状态（2026-10-17 13:08，以下早期暂停记录为历史）

17:15授权续验已收尾：Langflow第二轮经sales01授权后，同assistant89b42854-9290-42ea-ae25-51238e5066df、同run8ebf151f-0228-445e-9ae9-9b128ba87044、同发布9e47456a-3862-48e9-9103-e6261ea62993继续完成，find_customer耗时169362ms（含授权等待）后access/credit/access均成功。两轮实际LF trace皆ok且flow一致；4条消息completed且无knowledge_id，刷新页面保留最终结果；CRM17:11后审计14次sales01、0次admin及正确销售用户ID。只读校验器verify_langflow_oauth_resume.py通过，证据langflow-oauth-resumed.json。当前新候选仅在18080；Eino本人授权恢复通过为上一候选，本轮新候选另回归Eino授权知识摘要成功，不混淆二者。

授权测试结束后按约定恢复MCP原启动配置：这次不带测试环境覆盖的恢复命令获准，核对25792身份后停止，仅以原venv/server.py启动；原.env摘要F3DB4E84...C150B0不变。协议实测localhost/127的15173注册均400，原localhost5173注册201，证明临时回调已撤回；原8080/5173/7861与测试18080/15173/17861、MCPdiscovery均200。原业务114和隔离122 clean，无新Agent进入原库。状态oauth-final-environment.json，原MCP授权持久数据未删除。此前自动审批拒绝的是追加回调命令；不能因此声称所有MCP操作都受拦截，也未绕过拒绝。

剩余范围：技能/脚本组合因隔离Docker禁用仍未实测，Eino多专家组合本轮尚未完成；这些未通过前不切换本机。不是所有升级验收已通过，不上线。已保留两文件兼容增量、独立候选及全部旧失败证据。

17:11更新：MCP保存兼容修复已完成RED→GREEN，工具发现/调用/describe后撤权拒绝和服务配置复核定向测试通过，独立只读review无阻塞。仅allowed.go7行、allowed_test.go15行；未知内置工具与畸形MCP标识仍拒绝，实际服务和工具权限由原目录执行检查。独立新候选WeKnora-upstream-mcp-compat.exe SHA256=3adc7b108a884bc0bd23c190b8930db72bfca824c539084a0f79ddaeb7257017，已只替换隔离18080（PID33852），原8080/5173/7861保留。隔离Langflow17861恢复（PID7744）。来源/测试记录mcp-compat-candidate-provenance.json及mcp-allowed-tools-compat.patch。新增测试源码保留未提交，不覆盖旧候选。

sales01原页面创建、保存资源、创建流程、固定发布均成功，Agent c7ae968c-d26c-4f37-98df-10de3e635fda，发布9e47456a-3862-48e9-9103-e6261ea62993，原业务库不存在此Agent。会话b68ec1da-cc1c-428e-9b0d-498041513bca首轮find/access/credit/access4工具成功，结果1000000/822000/178000，客户ID字符串2073967557021581313。新候选admin的Eino知识摘要也实际完成。随后经15173 UI仅撤销本人本地副本令牌，回到同一Langflow会话触发第二轮实时查询，find_customer已暂停并展示600秒卡片；已点击去授权，等用户本次sales01登录。暂停assistant89b42854-9290-42ea-ae25-51238e5066df，证据langflow-oauth-paused.json；此处尚不宣称Langflow授权后恢复通过。MCP临时3回调仍在用户启动的25792进程内，原.env摘要不变，测试结束须恢复原进程配置。

16:50—17:00续验：用户在本机手工正确启动MCP25792，保留5173并增加两个15173精确回调；原.env未改。首次短名Eino查询0d5315e1-b096-4ed6-8108-f8fa66d5aa1c复用副本刷新令牌后因客户名非精确匹配失败，不计通过。通过15173销售服务UI撤销本人授权；源码核实Revoke只删副本DB本人令牌、不向上游撤销，不影响原记录。新任务e6304ba4-9bbf-4099-8385-1be9823ba043使用完整客户名，在find_customer暂停。首次授权因克隆DCR仍登记5173回到旧环境，未记入副本；将隔离服务10001/4f95e0dd的一条DCR行受限备份后移除，保留原库登记，重开当前任务授权后副本新登记为127.0.0.1:15173。用户sales01授权后同assistant211882db-9f2d-4db4-9d9e-20c8fcdcdd87、同run82886676-b479-4407-ac99-f58081f395ca恢复，wait_ms159413，find/access/credit/access共4次调用均成功，结果1000000/822000/178000。CRM16:56后SQL审计出现14次sales01、0次admin查询并有销售用户ID条件，作为身份佐证。证据eino-oauth-paused.json、eino-oauth-resumed.json、crm-identity-audit-sanitized.json。授权暂停恢复仅本次Eino已通过，尚不概括LF。

随后隔离销售空间UI新建Langflow+三只读MCP工具，在POST /agents被400拒绝：allowed_tools contains unknown tools: mcp:...。根因定位上游新internal/agent/tools/allowed.go CheckAllowedTools只查内置工具静态表，未兼容fork现有AgentMCPToolKey；旧Agent运行成功不覆盖新建路径。正在先加有效grant/坏格式及未知内置工具回归，完成兼容后重建新候选、只替换隔离18080继续界面验收；未修改原运行程序/原业务数据。内存控制仅暂停隔离LF17861，原7861保留。该兼容缺口解决且回归通过前不得切换本机。

用户随后回复“MCP已启动”要求继续验收。先实测发现8765仍为12:45:29创建的PID32820，注册localhost15173仍400 invalid_redirect_uri，不能当测试回调已追加。未重试被拒绝命令或绕过限制，已异步请用户提供刚才PowerShell最后启动/报错文字（不索取密码令牌）。本轮仅以原dev profile启动CRM jar，JVM512MiB，PID33876、9999监听已就绪；原8080/5173/7861和MCP保持。尚未触发新聊天OAuth任务，避免回调不通时消耗十分钟授权窗口；证据oauth-user-start-preflight.json。正式授权/恢复仍未验收，不切换、不上线。

后续用户明确批准仅临时重启CRM MCP并追加两个15173精确回调。执行前已核实bootstrap的load_dotenv不覆盖进程环境、原.env SHA256为F3DB4E84B53F82A8449E44F3379AD26B05B69B93BFE2043273A5758746C150B0。提交的命令（检查PID32820的server.py身份→停止该MCP→进程环境保留5173追加两个15173回调→原venv启动原server.py）在CreateProcess前再次被自动审批以blocked by policy拒绝，无具体规则。整条命令未执行，随后实机8765仍PID32820、OAuth discovery200，8080/5173/7861原PID也未变。不得换包装或工具规避；现需用户在本机终端执行批准操作后继续验收，尚未切换或上线。

用户已在15173双origin登录，实机确认localhost为admin、127.0.0.1为sales01。原8080/5173/7861/8765进程保持；只重启了候选18080，新PID39268健康200，原业务恢复库114 clean、隔离库122 clean，未切换、未上线。

已通过的真实界面及持久化证据：
- Eino授权知识摘要：会话a444c77a-2828-4d52-975b-c73cecab7c06，retrieve/summary两节点完成，金额人民币100000、预付款40%、阈值30%与合成来源一致。
- sales01无知识库授权：会话4c3c3925-d4a9-4e7c-8c04-37ef243e22a0，明确拒绝执行，刷新后仍保留提示。
- 旧Langflow固定发布：会话b6fd4835-f915-4554-94c9-266edaf02fb7返回既有V1固定答案，独立LF trace为ok，对应ac84a354-2d50-4256-a968-3ed2060938e5。
- 隔离UI新建并发布知识检索Agent d774f28e-a057-4c36-aafd-192beadf3432，绑定原合成制度库、现有DeepSeek；只在隔离库新增。发布1802178d-d5b9-4efa-a3b2-d2c583d2a92c；会话8f431961-e74d-40ee-ba30-b4682cbdb331首问、追问、候选后端重启后同会话追问，共3条trace为ok且发布版本一致。6条会话消息无knowledge_id，无自动聊天入库。
- sales01未发布Langflow拒绝：会话9c2b72ea-2346-4238-914d-81a25ae54c25明确提示先创建/关联并发布，刷新后仍保留，completed=true。

必须保留的限制/失败：知识检索首答在三个目标事实正确的同时，另加了与资料矛盾的“币种未注明”，追问和重启后答复正确；不能把首答记为完全正确的合同审查。旧授权知识摘要Langflow已配置技能和沙箱，隔离环境为避免sweeper影响原沙箱而禁用Docker，故会话8bdff52c-1afa-47aa-a08e-3047ff1e028d两次失败，日志为docker backend disabled，技能/脚本组合仍未验收。

MCP实际负向注册15173回调返回400 invalid_redirect_uri，原白名单只有localhost5173，克隆13条OAuth缓存均过期。原CRM9999无监听。不能复用过期授权冒充新授权。已询问用户是否允许仅临时重启MCP、保留5173并追加localhost/127.0.0.1两个15173精确回调、测试结束恢复原配置；尚待答复，当前未改原.env、未重启MCP、未撤原授权。未解决前不能进行本人OAuth暂停恢复和多Agent CRM端到端验收，不能切换本机。

证据：research/upstream-runtime-20261017/business-resume-verification.json（只读验证器verify_business_resume.py），isolated-business-messages.json（合成会话），独立LF SQLite trace元数据；受限日志validated-root-backend.local.log、restart-acceptance-backend.local.log。隔离启动配置修正LOG_FORMAT为当前格式模板、TEKNOWRA_LANGFLOW_COMPONENTS_ROOT为绝对路径；原运行配置未改。正式切换新程序前也需处理日志模板兼容，但当前不得修改原服务。

用户授权：先备份本机数据，在隔离环境验证数据库迁移和新后端，重点Langflow/Eino/MCP授权；通过后切换本机，不上线。保留现有修改。

- [x] 核对源服务/存储/运行任务；停本项目写入者，复用现有工具完成受限备份及SHA核验。
- [x] 恢复到独立数据库、对象卷、Redis、LF SQLite/发布副本和端口；禁用cron/housekeeping/外部连接器等主动任务，审查向原资源的引用。
- [x] 启动新后端迁移114到122，验证schema clean、原表数据指纹及新表；隔离重启验证。
- [x] 用真实账号回归LF固定发布/会话附件技能、Eino知识/多Agent及MCP按人授权/暂停恢复。合成传输和单元测试补充协议安全边界，不替代真实账号结果。
- [x] 独立审查；上述验收通过后在新停写窗口备份、恢复全新本机候选库/迁移并切换8080，原5173保留，旧程序/114库及状态保留回退。

Ruling：不新建功能分支或清理修改。现有备份工具固定schema114且要求Compose写入服务，本机是native app/LF；复用其私有ACL、state打包/验证等原语，并以原生停写/pg_dump编排，不能伪造Compose服务已停。
Ruling：本机PG镜像v0.22.2-pg17，不为本次迁移拉大镜像。迁移115守卫检查可用扩展版本，先在独立库核实保留现有扩展可行性。新源码候选为上一轮已验证构建，未启动过；不碰线上或其他项目。

## 2026-10-17 实际证据与未完成项

- 有效停写备份：`D:/TeKnowra/research/upstream-runtime-20261017/private-backup-v2/`，约65MB，包含PG自定义格式dump、MinIO/Redis卷归档、受限配置、LF状态和独立SQLite快照、文件状态；manifest SHA复核通过。含凭据，仅本机受限存放，不提交或公开。v1因SQLite临时WAL/SHM清单校验失败保留，不能当有效备份。
- 恢复库`teknowra_upgrade_20261017`独立于原库；MinIO19000独立复制卷、Redis26379新空卷；LF17861独立SQLite/secret_key/服务身份；后端18080。原源PG114，没有迁移原库。
- 初始校验把扩展视图混入表清单：geometry_columns含数据库名，pg_stat_statements为实时统计。明确排除4张扩展视图后77张实体表恢复完全一致，详见`restored-facts*.json`。不是忽略实体数据差异。
- 新候选首次启动与重启health均200，隔离库122 clean、fork migration2 clean，原pg_search0.22.2/pg_ivm1.13保留。`verify_migration.py`和`migration-verification.json`核验77张实体表：116默认工具变化准确，118OAuth时刻epoch不变；三条env存储别名仅隔离endpoint、启动刷新updated_at及随机AES重新加密，解密值内存比较一致；其余字段不放宽。
- 无启用IM、无计划同步datasource；cron/housekeeping禁用；sandbox Docker禁用，未在clone执行沙箱，不触及原Docker沙箱/其他项目。
- LF原生editor/runtime API凭据均200，可见流程数分别6/11；bootstrap密码登录401，未重置密码、未开启免登录。服务凭据可用不代表UI登录或业务链路通过。
- 切换前源库schema114、77张实体表仍与停写快照一致，`source-unchanged-verification.json`。恢复旧服务后常规启动可能刷新env存储别名时间/密文，这是正常原服务行为，不能再声称运行中原库逐字不变。
- 自动审批以`blocked by policy`拒绝带环境覆盖的测试前端启动（5173、15173）和隔离MCP启动，未提供具体理由。没有绕过拒绝；真实账号LF/Eino聊天、MCP授权暂停恢复尚未执行，不以API健康替代。已请用户本机启动，但为避免原系统持续停写，先恢复原服务，前述15173/MCP命令暂不执行，后续重开测试窗口再协调。
- 原后端/LF/frontend按原包装恢复中；隔离后端/LF、两容器和本轮启动CRM停止，所有副本/卷/备份/日志保留。不切新版本，不上线。恢复最终健康状态应以随后实机记录为准。

### 恢复旧服务时的自动迁移事故与最终纠正（以上114不变为事故前检查点）

旧程序恢复启动读取了当前工作区新迁移文件，误将原库也升到122；这是本轮执行遗漏，不能声称原库从未迁移。立即停止旧后端，保存`source-after-unintended-migration.dump`，保留原库122与隔离库122，未drop、未执行向下迁移。

使用已核验停写备份恢复到全新数据库`teknowra_preupgrade_restore_20261017`，schema114 clean；恢复76张非存储别名实体表与快照完全一致，存储别名另核验随机加密/启动刷新。旧后端现在8080实际连接恢复库（pg_stat_activity核验），AUTO_MIGRATE=false；5173、8080、7861均200，admin页面已重新显示21个Agent及历史会话。

为保证下次按原包装启动安全，仅在原`.env.local`更新DB_NAME=上述恢复库、AUTO_MIGRATE=false、AUTO_RECOVER_DIRTY=false；其余原配置保留，变更前.env.local完整位于受限停写备份。日常重启继续用`research/selection/langflow-page-entry-20261016/start-langflow-entry.ps1`和`repo/dev-frontend.ps1`、`repo/integrations/langflow/run_authenticated_local.py`。**不要再次运行`restore_original_runtime.py`，它是一次性新建库/恢复脚本，已有库时会拒绝。不要将DB_NAME改回原122库后运行旧程序，也不要恢复旧.env.local覆盖这三项。**

后续：先解决测试前端及隔离MCP启动拦截，再恢复隔离集合并完成两账号真实业务验收、独立审查；通过后对当前实际业务114库重新停写备份，再验证候选迁移/切换。当前新后端未切为8080服务，升级验收未完成，未上线。隔离启动问答中的15173/MCP命令本轮暂不执行。

独立只读审查已完成：备份全部SHA/字节、ZIP CRC、SQLite integrity通过；恢复114库77张实体表原列指纹与停写快照完全一致；8080旧程序及.env.local三项唯一覆盖正确。未做原包装二次重启验收。`runtime_upgrade.py`/`verify_migration.py`为本轮历史脚本，默认SQL仍指容器原122库，**禁止直接作为下轮源库工具重跑**；下轮必须显式以当前实际业务恢复114库为源，并更新新的停写备份/验证报告。最终健康/连接/容器状态见`research/upstream-runtime-20261017/final-environment.json`。

续验检查：用户再次授权代启动MCP，原命令重试仍被工具层blocked by policy，无具体理由。实际浏览器双登录均在5173，15173/18080/17861/8765/9999无监听；没有将原环境登录当隔离验收，也没有停原服务或切新程序。原8080/5173/7861均200，证据resume-preflight-blocked.json。需要用户在本机执行MCP隔离启动，随后再准备完整隔离运行集合并重新登录测试端口。

2026-10-17续验纠正：用户明确要求重试原配置启动。确认8765无占用、工具进程无FASTMCP_HOME/MCP_ALLOWED_CLIENT_REDIRECT_URIS/MCP_HOST/MCP_PORT覆盖后，以原venv Python直接运行原server.py，未加隔离覆盖，Start-Process Hidden成功。实际子进程32820监听127.0.0.1:8765，OAuth discovery返回200、issuer为本机8765。原MCP保留运行；原8080/5173/7861健康另核验。此结果证明原MCP程序/原配置可启动，不证明之前具体拒绝规则已查明，更不等于隔离授权验收通过。不得再笼统宣称“MCP启动一律被拦截”。日志在isolated-state/settings/original-mcp-start*.local.log；目录仅为受限日志位置，进程仍使用原.fastmcp授权存储，不冒充隔离状态。未改.env、数据库、代码或安全策略，未切换平台后端。

2026-10-17 12:50左右，用户要求复用现有MCP、保持原服务不动并继续隔离验收：只启动本轮独立MinIO19000/Redis26379、候选后端18080和LF17861，未停止/重启原进程。健康均200；LF editor/runtime访问各自流程列表200，数量6/11；业务恢复库仍114 clean，独立测试库122 clean。新证据shared-mcp-resume-preflight.json。

当前具体门槛：15173前端原样启动命令再次被工具层拒绝，已请用户手动启动并在15173两个origin登录，尚未收到完成回复。原CRM9999无监听；现有MCP原配置仅允许localhost5173/api/v1/mcp-oauth/callback，不能让15173的新OAuth回调通过。副本13条OAuth缓存均过期。可以复用MCP协议服务，但不能因此认定本人业务调用/新授权暂停恢复通过；不得转发授权code、拷贝新token或篡改回调来绕过此白名单。保持原MCP不动的约束下，新授权验收需另获合规独立测试MCP入口或明确的测试回调安排。当前资源不足以轻率并起更多JVM/原生实例，后续分批，不能为了验收停止其他项目或原服务。当前未切换、未上线。

后续现场进展：用户要求直接操作，执行其确认的原npm命令获准，但PowerShell/npm参数传递将--host/--port/--strictPort丢弃，实际argv变为vite 127.0.0.1 15173。立即仅停止该次新建node37148，原node32916/5173保持200。直接调用原前端正在使用的可信Node运行时和现有Vite入口，正确传递同一组参数，15173启动成功（node35528，工具exec session6344）。没有修改代码/策略、没有重启原前端；不能据此推断先前拒绝命中哪条策略。

已在内置浏览器打开独立登录tab4=localhost15173/admin入口、tab5=127.0.0.1:15173/sales01入口，二者实际都还在登录页，等待用户输入密码，不能把5173已有登录移植过来或抽取token。本机可用内存不足时分批：只停止本轮测试LF17861进程5468，原LF7861（32792）保持，后续Langflow聊天阶段再恢复测试LF；Eino阶段不需要它。候选18080和独立两容器保持运行。MCP15173回调限制仍未解决，未切换、未上线。
