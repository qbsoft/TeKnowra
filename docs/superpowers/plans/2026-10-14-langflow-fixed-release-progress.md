# SDD ledger — plan: D:/TeKnowra/repo/docs/superpowers/plans/2026-10-14-langflow-fixed-release.md

- Scope：用户已批准第一阶段固定版本发布与最小服务认证，沿用当前工作区；不提交/推送/生产部署。
- Base：b7c739b91；大量A-C/Eino/OAuth未提交文件保留。
- Pre-flight：现有唯一Langflow账号langflow为超级管理员，auto_login打开；官方普通账号Flow私有权限与API key可复用。单独复制图和hash不足以限定研发权限，需编辑/运行身份隔离。
- Ruling：服务端私有工件、仅发布生成不开放修改、运行前指纹检查；系统/OS管理员属可信运维，不声称抵御管理员改库。暂拒绝外部子流程与动态变量。
- Tasks 1—5 pending；源代码尚未修改。
- Task 1 implemented/GREEN：发布库不可覆盖、跨Agent激活拒绝、切换后的任务绑定副本、重启恢复与回退测试通过；坏摘要先RED后GREEN，持久写入失败不改变活跃版本与损坏库拒绝通过。Config草稿/活跃读取分离先RED后GREEN。
- Task 2 implemented/GREEN：认证客户端无auto_login、禁重定向、错误响应不泄密；运行前所有者/图摘要检查、未发布拒绝及release_id事件已接入。客户端/运行时测试通过，宿主启动凭据与VerifyRelease仍待任务3接入。
- Task 3 in progress：发布管理器的导入失败不切活跃版本先RED后GREEN；图校验拒绝未绑定资源、动态变量、临时能力、外部子流程。原生Agent同样校验源码，恶意替换原生代码测试先RED后GREEN。组件与支持模块/依赖版本指纹已实现；待服务/角色/UI实机验证。
- Ruling：图校验/发布生命周期放入langflowrelease包以便独立测试，服务文件仅准备权限与依赖，避免DI循环（agentService不能依赖含SessionService的发布/目录服务）。
- Ruling：服务端仍使用本机回环拓扑；认证使用普通编辑与普通运行身份。管理员/OS管理员可信，禁止在线修改运行工件；不宣传管理员攻击防护或跨容器部署完成。
- Task 3/4 implemented：发布/激活/列表服务接入现有可编辑Agent授权，目录仍读草稿；API路由与前端发布/试运行门控测试GREEN。已修正路由测试，必须同时断言实际注册与API key拒绝（仅断言403会被未匹配路由默认拒绝误导）。组件与支持模块、Langflow/lfx/模型SDK依赖版本纳入指纹。
- Task 2 metadata：错误元数据改从实际Engine副本取得，不重新读取活跃版本，防止任务期间切换后错误记录标错版本；RuntimeMetadata先RED后GREEN。每个步骤也写实际ReleaseID。
- Task 5 initialized：官方API创建teknowra-editor/teknowra-runner两个普通账号，两份独立编辑Flow；密钥只留研究环境data受限文件，ACL仅当前Windows用户与SYSTEM。原流程仍在，两个新账号没有超级管理员权限。
- Task 5 startup issue：关闭auto_login后官方要求SUPERUSER启动凭据而拒绝启动；按官方逻辑在已有受限本地身份文件配置原langflow管理员的启动种子，不新建/提升服务账号。官方逻辑只轮换仍使用旧默认口令的管理员，已有正常口令不会被覆盖。原管理员有last_login_at，原流程经只读DB确认保留。第二次认证启动进行中。
- Go回归GREEN：config/langflowrelease/langflowruntime/application-service/router（Langflow/Release/Component/旧Eino/Scope/APIKey选择）；前端门控2项通过。已启动本机后端重建，真实认证和版本发布UI验收待完成。

- Native review：独立审查无Critical，3项Important：原生Agent模型入口/附件、LangChain/LangGraph依赖遗漏、发布库写入大小无上限。均已增加失败测试并修复，发布包与认证客户端全量测试GREEN。新增正向宿主模型连线/重复连线拒绝测试。
- Live authentication：auto_login、匿名Flow访问、错误key均403；editor/runner均active且非superuser，runner读取编辑私有草稿404。后端本机认证模式已启动。
- Live graph compatibility：官方bundle类型ext:teknowra:前缀和template.is_refresh布尔元数据原被拒绝，已分别RED→GREEN。真实保存图+实际安装源码的可选fixture测试GREEN。复制草稿继承旧任务字段/全局引用已仅在两个独立草稿清除，并固化bootstrap清理，不更改旧图。
- Frontend verification：发布门控2测试GREEN，vue-tsc --build退出0。Python组件7测试GREEN。错误提示显示宿主返回的安全诊断，便于定位被拒绝的发布。
- Task 5 pending：真实V1/V2发布、草稿隔离、回退、两场景正式聊天及运行工件编辑拒绝。不得将未通过的UI发布记为完成。

- Task 5 live GREEN：摘要V1草稿隔离、V2发布、界面回退V1，三次真实知识聊天与发布ID/运行Flow一致；合同与信用正式固定发布，两个Agent、五个工具span、真实客户ID与信用值完整。匿名/错误key/auto_login403，跨私有草稿与运行工件读写404。认证与四次聊天证据独立保存，未覆盖A-C材料。
- Verification limitation：并发活跃切换后的绑定固定由Store/Engine测试覆盖，不推断为真实暂停并发验收；原图片占位与模型前言问题保留，不扩本阶段范围。
- Remaining：最终Go针对性回归与Vue类型检查正在收尾，报告已记录实际通过项及后续范围。

- Final GREEN：Go config/langflowrelease/langflowruntime/application-service/router针对性回归exit0；langflowrelease与langflowapi全量包测试exit0；Vue vue-tsc --build exit0；前端发布门控2测试、Python组件7测试通过。Tasks1—5本机范围完成，无提交、推送或生产部署。验收记录：research/selection/langflow-decision-test-20261013/固定发布与最小认证验收记录.md。
