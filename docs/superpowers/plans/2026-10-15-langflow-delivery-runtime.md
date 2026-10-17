# Langflow 成组运行与版本包实施计划

> 当前工作区使用 superpowers:executing-plans 连续实施，保留全部未提交修改，不提交、不上线。

**Goal:** 提供可校验、安装、切换和回退的集成版本包，并在独立环境验证真实Langflow运行与拓扑边界。

**Architecture:** 单实例共享回环拓扑；白名单组件+依赖指纹+不可变工件声明组成版本；状态及凭据与版本目录分离，先校验再原子切换。

**Spec:** docs/superpowers/specs/2026-10-15-langflow-delivery-runtime-design.md

**Tech Stack:** Python标准库、现有Langflow/httpx、Docker Compose、既有Go执行协议。

## Tasks

- [x] 1. TDD发布工具：白名单打包/验证、zip路径与symlink/成员/大小校验、不可覆盖安装、原子激活回滚、环境指纹不符拒绝。
- [x] 2. TDD启动器：独立数据根、精确版本/源码检查、禁止auto_login/注册/全局变量环境导入，外置受限身份；生成容器共享网络模板和配置说明。
- [x] 3. 生成真实本机版本包；在全新安装根安装A/B、切换与回退/篡改失败，保留证据。不得把native包冒称完整Linux镜像。
- [x] 4. 暂停自有旧验收进程并记录恢复方式，启动独立LF/SQLite/账号，受控组件真实调用；Docker共享网络探针。完成后恢复原服务，记录未完成层级。
- [x] 5. 一次fresh只读审查，重要项TDD修补，最终回归、交付说明和限制。无上线。

## Ledger

- Preflight：D盘34.2GB，C盘10GB，可用RAM约0.43GB；Docker根在既有位置。禁止大规模拉镜像/构建同时占满内存。
- Ruling：共享命名空间保留loopback校验和临时能力端口，避免增加远程能力网关。成本：单实例成组生命周期，首版不独立横向扩容。
- Ruling：真实native测试与Docker网络验证分层记证据；没有当前未提交代码对应的Linux镜像时明确阻塞，不用现有旧镜像替代。成本：完整容器栈可能仍待构建环境。
- Builder check：fresh只读设计检查无必须补问；用户已授权当前工作区继续实施，不额外停下来确认。
- Tests：打包/启动器从缺实现RED→GREEN；未知安装成员/版本目录symlink另行RED→GREEN。首批8项、加固12项通过。
- Real startup：旧7861任务LF已暂停，既有平台8080/前端/CRM/MCP保留；新17861独立SQLite与普通editor/runner身份。首次bootstrap Fernet格式错误400已分类定位，启动前增加失败用例并修复；未改旧身份。先停完整自有launcher进程树，再核验端口归属，避免旧worker健康检查假阳性。
- Auth：清除测试脚本bootstrap Cookie再测普通API key（避免Cookie优先级污染）；实际私有工件隔离通过。准备脚本只读旧合成图结构并删除知识/技能节点，替换为纯合成模型，不复制旧服务凭据。
- Real Go runtime：新gated TestDeliveryIsolatedRealLangflow使用真实LF+实际Go Engine+合成宿主ChatStream，C、D和回退C均成功，模型每次1调用，无外部供应商/业务访问。
- Network：Docker仅用两只本轮专属label的现有sandbox镜像探针，共享namespace、动态回调和缺能力拒绝通过，无发布端口，已清理。不是LF镜像，也不是完整平台容器栈。
- Final review：唯一fresh只读审查3项Important。解释器/库错配拒绝、容器配置从活跃manifest锁定镜像/组件/profile、状态不得写入任何历史版本，均先RED后GREEN；同时拒绝平台和LF复用环境文件。15项交付测试通过，无循环复审。
- Artifacts：native C→D→C真实回退，最终E含全部修补，安装根/工件均在D:/TeKnowra/research/selection/langflow-delivery-test-20261015。包明确native-validation，不冒称Linux交付包。
- Container blocker：DockerHub langflowai/langflow:1.12.5 manifest查询EOF；现有本机app镜像也不是本轮未提交代码的不可变构建。没有拉大量镜像/全栈构建，没有用旧镜像顶替；完整Linux成组安装和回退尚未验收。
- Update：相隔一段时间重试后成功取得LF Linux amd64镜像digest `sha256:e5540a56a38ab34fcff604996c5d7797b88d16e444bd7ceddead077c5e277ac6`，仅元数据，未拉取运行。不能再把初次EOF当作当前永久网络阻塞；剩余关键条件是本工作区对应Linux app构建工件、匹配的Linux LF依赖profile及完整容器验收。
- Final native：E包SHA256 `d693443c5df763dd8fd53620b98ea3c806e5ac43df30f72ac4300b3aeaa30d40`；完整runtime测试14项、Python组件/技能/交付测试25项PASS。真实LF共4条ok trace（C初始、D升级、C回退、E最终），每次宿主合成模型1调用，没有外部模型或业务调用。版本校验/原子指针保护、native拒冒充container均验证。
- Restoration：隔离17861整棵自有进程树已停止；旧7861恢复，最终LF PID31664、8080 PID25024、5173 PID18536。旧数据/凭据/流程保留，不改生产。`delivery-evidence.json`记录完整证据和未完成层级；没有宣称Linux容器栈完成。
- Environment question：已向用户询问是否有可用的非生产Linux/CI环境。依据是AGENTS禁止生产服务器构建、本机实际资源限制；不是重复请求实施授权。等待环境信息期间，已完成上述原生验证并恢复原服务。

## 本机 Linux 分批构建续轮（用户确认无外部构建机）

- [x] L1. 拉取并核验固定 LF amd64 镜像，导出镜像内 Linux runtime-lock/profile；记录真实 entrypoint 与体积。
- [x] L2. 仅从白名单源码快照串行构建 browserskill、Go/Anydoc 与最终 app；保留完整解析功能，不拿旧 app 二进制代替当前源码。
- [x] L3. 具备真实镜像后生成 container 包并隔离验证安装、运行、升级/回退；前端与业务全栈验收另须当前版本对应工件，禁止将合成执行冒称完整业务验收。
- [x] L4. 恢复旧 LF，汇总资源与真实证据，明确未完成项；不提交、不推送、不上线。

- Resource ruling：Docker VHD配置位于D:/Program Files/Docker/data，起始D约32.08GB、C约10GB；固定LF压缩层总量约764MB。所有快照/日志/profile留D，D保留至少12GB、C保留至少6GB。重型阶段只运行一个，开始编译前要求至少1GB宿主可用RAM；低于0.5GB不得再启动新重型任务。达不到时保留已得工件和诊断，不通过全局清理或停止其他项目强行构建。
- Build ruling：原Docker忽略规则未覆盖全部本机环境文件，因此独立白名单快照构建；3181个源码文件，约27.54MiB，逐文件SHA保存linux-context-manifest.json。构建Dockerfile只在研究快照增加Go并行1、GOMAXPROCS=2、GOMEMLIMIT=768MiB、Rust jobs=1，不修改产品Dockerfile，不关闭Anydoc功能。成本：新COPY层可能失去旧缓存，必须按实测控制。
- Pause：核实24740为旧LF虚拟环境launcher且命令含langflow.exe/7861后停止其完整子树，释放本任务内存；平台8080、前端5173、CRM和基础设施保持运行。旧LF恢复入口为既有run_authenticated_local.py及原environment，不复制密钥。
- Resource ruling update：源码与Compose标签确认WeKnora-neo4j-dev归属D:/TeKnowra/repo开发栈，约886MiB；当前不做图谱验证，允许有序暂时停止该开发容器以腾出构建资源，结束后必须恢复。仅stop/start，不删除容器/卷、不变restart策略，不操作其他项目。成本：这段时间本机图谱功能不可用。
- L1 complete：LF Linux amd64固定digest镜像已实际拉取，image config ID为sha256:52aa5b3e67f090c4e232d891dfa180de93ca157059af6972109885d9371bba7f，实际展开2495571486字节；entrypoint为空，Cmd=[langflow,run]，工作目录/app。Python3.14.7；导出441项依赖METADATA+3关键源码至linux-profile，逐项与镜像内fingerprint相同；linux-runtime-lock.json为真实Linux捕获，不复用Windows锁。关键源码SHA与原生相同，但依赖版本集合不同。
- Linux tests：固定正式LF镜像中，交付15项、全组件/技能/交付25项均通过；容器受限、专属label、运行后自动删除。仅测试通过，不冒称平台全栈完成。
- Transport ruling：原HTTP Debian源连续出现502，已确认同一失败包HTTPS可200。只在研究构建快照导入固定LF镜像certifi的公开CA根证书，切HTTPS并加Acquire::Retries=3，保留TLS及Debian签名验证。原软件源逻辑不改；容器中LF使用Fedora信任存储，不存在Debian默认CA路径，首次导出路径错误已纠正，生成步骤加非零退出检查。HTTPS依赖安装通过。
- Helper evidence：现有旧app镜像BrowserSkill确实为0.2.1，当前源码要求0.3.1/da6bf4e；拒绝直接复用旧helper冒充当前版本。完整重构建，研究快照两个Rust阶段均限制jobs=1。
- Build provenance：白名单源码snapshot=2ca54ddd81e8aa634a301d5abcbe281a91795fd70c0ca0c0868f7b96ff9113c2，独立Dockerfile.app.delivery将migrate@latest锁至go.mod已有v4.19.1；当前正在执行的浏览器阶段文件不改动，后续平台阶段使用delivery文件。源文件hash与研究Dockerfilehash分别保留，不拿Git HEAD冒充脏工作区产物。
- Browser stage complete：构建退出0，专属镜像teknowra-delivery-browserskill:20261015，ID sha256:bc31989614a5d180f2ed1fa5634070be4d11c04fb0179d560c93ed87f1619023，2028415057字节；受限容器实际执行bsk --version=0.3.1。
- Build ruling update：Docker可执行文件所在目录未在会话PATH，导致浮动Go基础镜像查找docker-credential-desktop失败；仅补当前构建进程PATH，不改系统或凭据文件。已取得并锁定Go amd64基础sha256:bcef992b77b1e2031aaaa51da75cebc32da8e9c182e12e633ea79023c79d9eff，runtime Debian锁已有sha256:d5d3f9c23164ea16f31852f95bd5959aad1c5e854332fe00f7b3a20fcc9f635c。独立Dockerfile.linux-bounded在context外，Go资源限制前移覆盖工具安装与DuckDB编译，所有重型阶段串行。
- Download ruling：原DuckDB构建工具默认HTTP扩展下载同样502；只在镜像构建的下载工具阶段使用官方HTTPS仓库，并共享既有Go模块缓存，保留DuckDB签名校验；随后COPY覆盖回原下载源码，平台编译不包含下载器改动。官方依据https://www.duckdb.org/docs/current/extensions/installing_extensions。HTTPS下载成功，不减少spatial/excel扩展。
- Text ruling：许可证modules.tsv含2行CRLF，bash checksum正则把CR视为hash字符，先复现Invalid source checksum，Linux快照仅将该清单转LF后源归档校验均OK。产品原文件不改，原始与归一化SHA均保存。最终源码snapshot=b02ffc2efbff6f427f7fc6fab3dbc0f7c13afb636fecd2060fa56e5df2477415；独立构建配方SHA另存linux-build-recipe.json。
- Anydoc complete：原记录normalized日志中Rust release构建成功，332.8秒完成，产出lib/linux_amd64_gnu/libanydoc_go.a；Go随后报docreader/client与proto缺包。这是本轮白名单遗漏，非产品依赖缺失。补5个Go源码到独立linux-generated-go named context，在Anydoc之后COPY，避免重复Rust构建；补充源码hash见linux-build-source.json，完整source ID=00ab877eaf0bd13b32e53603bf2c35bc6586e9b21d2b05b2d85b80878b4285a6，实际编译COMMIT_ID覆盖为该source ID，之前context ID仍保留。
- Resource ruling correction：仅用Windows FreePhysicalMemory把WSL已分配文件缓存误判为耗尽。实测Docker VM MemAvailable=5370736KiB（约5.12GiB）、Cached=4489960KiB，Host Free约0.85GiB；后续容器内编译可回收现有VM缓存。改为同时要求VM MemAvailable至少2GiB、Host Free至少0.5GiB，继续保留C6GiB/D12GiB底线与Go768MiB/-p1/Rust jobs1限制；不执行全局drop_caches、不改WSL设置、不停其他项目。成本：单并发构建较慢，仍须持续监控宿主与VM两层。
- Linux app compilation complete：builder退出0，镜像ID sha256:cad3106f583bda3724b045bbf40c94fed530e739a1d29699ba32cd3fc61b1bb6。Go1.26.9/linux-amd64，真实二进制SHA256 af5341cd726dd6eb1112f97e88c9c8091b47a606e31e2cf0054191169070f8b4；Anydoc静态库SHA256 721e2ebff855fe8a6c2f7e54f6a9f7915b9ead5772bbca9953ad2346da05b378。go version -m确认anydoc、linux/amd64与workspace-00ab877eaf0bd13b，ldd无缺失库。元数据BuildTime仍unknown，GoVersion显示系列1.26，外部构建证据记录实际1.26.9；不可声称已达到全部正式发布要求。
- Cache ruling：内存短时低至0.23GiB，未并行起重型任务。仅对本轮不运行LF镜像的/app/.venv与新browser镜像的/usr/local/{rustup,cargo}、/opt/weknora/browserskill，使用只读FD的POSIX_FADV_DONTNEED提示回收自身文件缓存；未做全局drop_caches/清镜像/删数据。hint文件大小不等于实际回收量。宿主空闲曾回升0.68GiB，VM仍约5GiB可用。

- 完整app构建成功，原二进制SHA=af5341cd726dd6eb1112f97e88c9c8091b47a606e31e2cf0054191169070f8b4。后续仅补迁移层，未改Go二进制。完整迁移app config ID=c226f1cdbe7143c360c0fbb5d9914e334f6e6f905fd480e9cbcdb8283fa9b528；本机registry digest=f0ad22a7a774b424873509658dba1601bddc0fc31d972a6b625e8b09eb6e3791，不是外部发行。
- 根因纠正：研究白名单通过credential/token等文件名过滤误排35/54两组合法SQL。初判源码缺迁移错误，fresh只读审查发现原35已存在。临时115源文件撤销，完整305个SQL独立冻结，镜像逐文件SHA相等。新delivery-pgdata-complete卷从零迁移114/clean，注册201、登录200、身份200；旧115证据卷保留，未删/降级。PG回归测试保留，验证原始全迁移可创建真实Tenant并写读JSONB，绝不把115误诊当产品修复。
- 启动根因：首次新PG初始化时app连接拒绝后自动重启，LF先启动留下旧namespace；容器ID匹配不能证明实际namespace一致。Compose增加app健康检查和LF service_healthy依赖，研究切换脚本显式先等待app健康再起LF。运行中独立重启的成组恢复未实现，不声称高可用或无人值守交付完成。
- 最终G→H→G三轮实际容器ID不同、proc网络namespace同组一致，原平台登录、普通runner与私有图隔离保留；各轮Go Engine→真实LF→合成宿主模型1调用成功。14项Go运行时、25项Python和1项PG凭据回归通过；外部模型/CRM/企业知识未调用。G/H只有组件注释标记差异，app/LF镜像相同，不能据此声称跨二进制/依赖/schema回退兼容。
- A–F过程包不作交付候选；最终包G SHA=50574a37f707681803c4a56bb1d0e9672a582c2a170b18a2a1ed00b2d038ac78、H SHA=5529fcad8eed84d0379871aa1c26edd35723ebea2b8fbf33b9557d90ee3dac35。研究根linux-build-evidence.json记录最终分层证据，交付约束见docs/superpowers/langflow-linux-delivery-validation-2026-10-15.md。其余阻塞：自动成组故障恢复、跨机受控镜像分发及复现构建、对应前端和真实企业Linux业务验收、备份恢复/跨版本兼容。
- 恢复已验：本轮隔离app/LF/PG/Redis与本机registry均有序stop，17861/18080/15000不再监听；两只测试PG卷保留。原LF7861已恢复（launcher32644，监听33316），/health_check=200；平台8080/health=200、前端5173=200，CRM9999/MCP8765原端口仍监听，原Neo4j容器已start/running。未声称恢复期间所有原业务回归已测。C约9.40GiB、D约21.45GiB；restoration-evidence.json保存恢复证据，未操作生产和其他项目。
