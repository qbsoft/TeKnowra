# 品牌改造清单（TeKnowra）

本仓库是 `Tencent/WeKnora` 的 fork。品牌改造**必然要动上游文件**，而动过的每一处
都是将来合并上游时的冲突点。这份清单记录**动了哪些、为什么、以及冲突了怎么办**。

原则：**能换资源就不改代码。** 改动集中在尽量少的文件里，且优先选上游改得不勤的地方。

---

## 一、零代码改动（换资源，几乎不冲突）

| 文件 | 做法 |
|---|---|
| `frontend/src/assets/img/weknora.png` | 直接替换为 TeKnowra 文字图（400×126，透明底）。**路径不变**，引用它的 `menu.vue` / `Login.vue` 一行没动 |
| `frontend/public/favicon.ico` | 同上，32×32 |

`menu.vue` 近半年上游有 50 次提交，是全仓最勤的文件之一。走换图这条路，
它完全不在改动清单里。**将来加品牌元素优先考虑这个办法。**

只有上游哪天换了自己的 logo 文件才会冲突，那是二进制冲突，取本地版本即可：

```bash
git checkout --ours frontend/src/assets/img/weknora.png
```

---

## 二、改了上游文件（升级时要重新应用）

| 文件 | 改了什么 | 上游近半年提交 |
|---|---|---|
| `frontend/index.html` | `<title>` 与 description 改为 TeKnowra | 低频 |
| `frontend/src/views/auth/Login.vue` | 去掉 3 个外链（上游 GitHub ×2、官网 ×1），logo 外链改为 `div` | 24 次 |
| `frontend/src/views/settings/Settings.vue` | 移除「WeKnora Cloud」导航项 | 28 次 |
| `frontend/src/components/ModelEditorDialog.vue` | `HIDDEN_PROVIDERS` 过滤掉 `weknoracloud` | 中频 |
| `frontend/src/views/settings/ParserEngineSettings.vue` | 引擎列表过滤掉 `weknoracloud` | 中频 |
| `internal/application/service/embed_webhook.go` | 签名头 `X-WeKnora-Signature` → `X-TeKnowra-Signature` | 低频 |
| `internal/middleware/auth.go` | JWT `aud` = `teknowra` | 36 次 |
| `internal/handler/tenant.go` | 签发 JWT 的 `aud` 同上 | 中频 |
| `internal/middleware/auth_api_principal_test.go` | 测试里的 `aud` 跟着改（4 处） | 低频 |
| `internal/sandbox/docker_engine.go` | `ValidateDockerHost` 放行 `npipe://`（Windows 本机守护进程），并把它加进缺少 scheme 时的报错提示 | 见下 |

### Windows 上 Docker 沙箱要放行 `npipe`

不改的话 Docker 后端在 Windows 上**完全不能用**：沙箱配置的守护进程地址留空时，
`DetectLocalDockerHost` 去读 docker CLI 的上下文，Windows 返回
`npipe:////./pipe/docker_engine`，而 `ValidateDockerHost` 只认
unix / tcp / http / https，于是配置在保存前就被拒：

    sandbox: unsupported docker host scheme "npipe"

界面上只显示「检测未通过」，看不出是平台不支持还是自己填错了。

**只需放行，不需要写传输层**：本包用的是 moby 官方客户端，它自己就认 npipe；
拨号守卫只管网络端点、TLS 校验只管 tcp/http/https，命名管道跟 unix 套接字
一样是本机端点，两者本来就不该管它。

**已提给上游**：[Tencent/WeKnora#2846](https://github.com/Tencent/WeKnora/pull/2846)
（分支 `upstream-npipe`，基于上游主线单独摘出）。合并后可以从本清单里删掉这一节。

### WeKnora Cloud 为什么要三处一起堵

它不只是个设置页，还是**模型供应商**和**解析引擎**的一个选项。只藏设置页的话，
用户还能从「模型配置 → 新增模型 → 供应商」里选到它，然后去申请一个本产品
根本没有的云凭证。三个入口：

1. `Settings.vue` —— 左侧导航项
2. `ModelEditorDialog.vue` —— 模型供应商下拉
3. `ParserEngineSettings.vue` —— 解析引擎列表

---

### 按 agent 的 MCP 工具授权，钩在上游目录的两个卡口上

上游 2026-09 把 MCP 从「逐个注册进工具表」改成了「目录 + 按需调用」
（`feat(mcp)!: persist tool catalogs`、`refactor(agent): discover and invoke
MCP tools on demand`）：注册表里只剩一个发现工具和一个 `call_mcp_tool`，
具体工具由模型运行时 describe 出来。我们原来「注册完再按 allowed_tools
清扫注册表」的做法从此扫了个寂寞——没有东西可扫，等于全放行。

现在的做法：`internal/agent/tools/mcp_catalog.go`（上游文件）加了三小段——
`MCPCatalog.agentAllow` 字段、`visibleTools` 里一条过滤、`checkEnabled` 里
一条复核。这两个函数是所有发现路径和**所有执行路径**共用的卡口（模型自己
拼 tool_ref 也绕不过 `checkEnabled`），跟上游自己的空间级 enabled 策略在
同一处生效。入口和键格式在我们自己的文件里：
`internal/agent/tools/mcp_agent_allowlist.go`。

授权键从注册名换成了 **`mcp:<服务ID>:<工具名>`**。注册名（`mcp_mail_send_email`）
的服务段来自服务显示名的有损消毒（中文名直接消没），新版还掺了 schema
哈希——改个名、改个参数定义都会让存下来的授权静默作废，都不配当存储键。
服务 ID 是 UUID，天生稳定。旧格式条目不再授权任何东西，后端启动 agent 时
会打 Warn 提醒重新保存工具勾选。

升级时若这三段钩子被上游改动冲掉，`internal/agent/tools/mcp_agent_allowlist_test.go`
会红（发现路径漏（TestAgentAllowlistFiltersDiscovery）、执行路径漏
（TestAgentAllowlistBlocksCallsEvenAfterDescribe）都有断言），照着测试补回即可。

### 提示词里的品牌：加载时替换，不改 yaml（2026-09-18）

上游 `config/prompt_templates/*.yaml` 的每个人设都以 "You are WeKnora, ...
developed by Tencent" 开头，模型会照着它向用户自我介绍（问「你是谁」就答
「腾讯开发的 WeKnora」）。第一轮品牌改造漏了这一块。

做法：`internal/config/branding_teknowra.go`（我们自己的文件）在模板读进来时
把 `WeKnora` → `TeKnowra`、`developed by Tencent` → `developed by TyerSoft`；
上游文件 `config.go` 的 `loadPromptTemplates` 里只挂了一行调用。yaml 本身一字未动
——它是上游改得很勤的文件，逐行改每次合并都要冲突；而且上游新增的人设会被自动换掉。

只替换大小写完全一致的 `WeKnora`。小写的 `.weknora/requirements.json` 是技能机制
真实依赖的路径，不能动。合并上游后那一行钩子若被冲掉，
`branding_teknowra_test.go` 的 `TestLoadPromptTemplatesAppliesRebrand` 会红
（它用仓库里真实的模板目录走一遍加载）。

注意：智能体一旦在界面上保存过，提示词就落进数据库 `custom_agents.config`，
不再读模板。库里的旧提示词要另行处理（目前只有内置的「技能安装器」还带着，
用户看不到）。

### 指向上游仓库的入口：一个开关 + 一条全局样式（2026-09-18）

| 入口 | 做法 |
|---|---|
| 用户菜单的「帮助与文档」「GitHub ★」 | `UserMenu.vue` 用 `SHOW_UPSTREAM_LINKS` 开关包住，代码保留 |
| 设置里的「版本信息」页 | `Settings.vue` 把 `platform` 分组清空；页面仍可用 `?section=system` 打开 |
| 各设置页的「查看文档/指南」链接（7 个文件） | `frontend/src/assets/teknowra-branding.css` 一条全局规则隐藏所有指向 `github.com/Tencent/WeKnora` 的链接，`main.ts` 引入；那 7 个上游文件一行没改 |
| 靠点击打开的两处 | 知识图谱指南的 `.graph-guide-link` 类名唯一，样式直接选中；API 文档链接在 `ApiIntegrationSettings.vue` 补了 `.upstream-doc-link` 类 |
| 「CLI」「Claw Skill」两个集成页 | `frontend/src/config/integrations.ts` 的 `HIDDEN_INTEGRATION_TABS` 把它们从导航列表里滤掉（设置导航和侧栏悬浮预览共用这一个列表）。它们介绍的是上游发布的外部产物，本产品没有对应物。`INTEGRATION_TABS` 没动，页面仍可用 `?section=` 直接打开 |

### 其余用户可见的字符串（2026-09-18）

| 位置 | 用户在哪看到 |
|---|---|
| `browserskill/authorization.go` 的 `service_name` | Chrome 里任务标签组的名字 |
| `mcp/oauth_manager.go` 的 `clientRegistrationName`、`mcp/client.go` 的客户端名 | 对方系统（如 CRM）的 OAuth 授权确认页；只影响之后新注册的客户端 |
| `im/feishu/adapter.go`、`im/service.go` | 飞书卡片标题、IM 里的授权提示 |
| `handler/sandbox_check.go` 两处 | 沙箱自检的提示文案 |
| 语言包 `windowHint` / 沙箱密钥说明 / `dockerHostRisk`（中英韩俄） | 设置页说明文字 |
| `FAQEntryManager.vue` 的示例条目、`chunkingSamples.ts` 的示例文档 | FAQ 示例、分块预览的样例（连同里面的上游仓库和镜像地址一起换成了示意地址） |

### 共享智能体的 MCP 按人授权：授权接口要跟着换到源空间（2026-09-19）

A 空间把挂了按人授权 MCP 服务的智能体共享给 B 空间的用户时，对话运行时平台用源空间（A）
解析模型/知识库/MCP，OAuth 令牌也按「源空间 + 本人」查；但授权这几个接口（取授权地址 /
查状态 / 撤销 / 确认 / 取消）按的是请求方自己的空间（B）——B 里没有这个服务，对话里弹出的
授权卡片点了就是 `MCP service not found`。共享出去的按人授权智能体别人永远用不了。

做法：这五个接口接受可选的 `?agent_id=&agent_source_tenant_id=`，带了就换成源空间；不带则
与上游行为完全一致。逻辑在我们自己的 `internal/handler/mcp_oauth_shared_agent.go`，
上游文件 `internal/handler/mcp_oauth.go` 里每个接口只改了取空间那一行，外加构造函数多收一个
`AgentShareService`。前端 `McpOAuthCard.vue` 在用共享智能体时带上这两个参数
（`api/mcp-service.ts` 的四个函数各多一个可选参数）。

安全边界：放行的是「对别的空间的服务发起授权」，所以必须同时满足——①请求方空间确实被共享了
这个智能体（复用平台的 `GetSharedAgentForTenant`）；②这个智能体确实用了这个 MCP 服务
（否则拿一个共享智能体当钥匙就能授权源空间里任意服务）。任一不满足即 403，绝不回退到源空间。
令牌仍记在请求方本人名下。

合并上游后那几行若被冲掉，`mcp_oauth_shared_agent_test.go` 的
`TestMCPOAuthEndpointsUseSharedAgentTenant` 会红；`TestOAuthTenantFor` 覆盖全部放行/拒绝分支。

### 启动时清掉上一个进程留下的「会话正在运行」标记（2026-09-20）

每轮对话开始时在 Redis 写一个 `<prefix>:<sessionID>:live-run` 标记，结束时在这一轮自己的 defer 里清。
进程退出时还没结束的那一轮（典型：停在「等待授权」，最长 10 分钟）来不及清；标记有效期 1 小时且每次读取
都续期，于是该会话此后一直 409 `another turn is already running in this session`，用户只能新建对话。
每晚自动更新、白天手动部署都会撞上。

我们是单实例部署：进程刚启动时不可能有对话在跑，启动那一刻的残留标记全是上个进程的，直接清掉。
逻辑在我们自己的 `internal/stream/liverun_sweep_teknowra.go`；上游文件 `internal/stream/factory.go` 里
一行钩子（`withStartupSweep(NewRedisStreamManager(...))`），同目录测试盯着。
**改成多实例部署时必须设 `TEKNOWRA_SWEEP_LIVE_RUNS=false`**，否则会清掉别的实例上正在跑的对话的标记。

### MCP 授权的回跳地址只认一个，由服务端决定（2026-09-20）

平台对每个 MCP 服务只向对方登记一次，回跳地址随登记一起交过去；而网页发起授权时回跳地址是前端用
`window.location.origin` 拼的——用户这次从哪个地址进平台就是哪个。换个地址进来（域名 / 内网 IP /
localhost / 127.0.0.1）授权就被对方拒掉，用户看到一屏 JSON（`Redirect URI ... not registered for client`
或 `does not match allowed patterns`）。

改成服务端说了算：已登记过就用登记时的地址；没登记过且配了 `APP_EXTERNAL_URL` 就用它拼（与 IM 机器人
生成授权链接同一口径）；都没有才用前端报的。逻辑在我们自己的 `internal/mcp/oauth_canonical_redirect.go`，
上游文件 `internal/handler/mcp_oauth.go` 的 `AuthorizeURL` 里一行钩子（网页和嵌入页都走这个函数）。
被冲掉的话 `internal/handler/mcp_oauth_canonical_redirect_test.go` 会红。

### 嵌入页按宿主用户分开存授权和对话（2026-09-19）

设计与取舍见 `docs/embed-per-user-authorization.md`。嵌入页在浏览器里按「渠道」存访客编号
（MCP 按人授权的令牌挂在它名下）和当前对话的指针，整个浏览器只有一份——同一台电脑上换个人
登录宿主系统，用的还是上一个人的授权和对话。宿主在 `WeKnora.init({ hostUser })` 里报上当前用户，
嵌入页把它拼进这两样东西的存储键。没传 `hostUser` 时键与上游逐字节一致。后端不改。

逻辑在我们自己的 `frontend/src/api/embed/hostUser.ts`。上游文件里的钩子（各一行）：

- `frontend/src/api/embed/index.ts`：`embedVisitorStorageKey`、`embedChatSessionStorageKey`
  末尾拼 `embedHostUserSuffix()`；`onEmbedHostToken` 里在调 handler **之前**调
  `setEmbedHostUser(e.data.host_user)`（顺序要紧：bootstrap 一进去就读访客编号）。
- `frontend/public/weknora-widget.js`：`provide_token` 消息带上 `host_user`。

平台生成的三种嵌入代码也带上了这个参数（2026-09-21）：`api/embed/index.ts` 三个 `build*Snippet` 各一行
（`embedHostUserAttr()` / `withEmbedHostUserPlaceholder()`）；`useEmbedBridge.ts` 的 `start()` 里一行
`applyEmbedHostUserFromLocation()`（iframe 方式从地址读）；`weknora-widget.js` 自动初始化读 `data-host-user` 一行；
`AgentEmbedChannelPanel.vue` 代码框下面挂一行我们自己的说明组件 `components/teknowra/EmbedHostUserHint.vue`。

顺带：授权弹窗走完后落在不带 token 的嵌入页地址上，上游显示「缺少嵌入渠道或 Token」，像报错。
`frontend/src/api/embed/oauthLanding.ts` 认出这种落脚并改成「授权成功，可以关闭此窗口」；
钩子是 `composables/useEmbedBridge.ts` 的 `start()` 开头一行。

合并上游后钩子若被冲掉，`hostUser.test.ts` / `oauthLanding.test.ts` 会红
（`cd frontend && npx tsx --test src/api/embed/*.test.ts`）。

### 浮窗启动按钮的图标（2026-09-20）

上游用 emoji 字符（💬 / ✕）当图标，长相由操作系统字体决定，Windows 上是个带灰影、发虚的气泡，
压在纯色圆底上很难看。`frontend/public/weknora-widget.js` 顶部加了一小块 fork 自己的代码
（`tkLauncherIcon` / `tkLauncherStyle`，内联矢量图标 + 居中 + 悬停放大），上游代码里钩子 3 处
（搜 `tkLauncher`）。被冲掉的话 `frontend/src/api/embed/widgetLauncher.test.ts` 会红。

## 三、刻意**没有**改的

| 类别 | 量 | 不改的原因 |
|---|---|---|
| Go 模块路径 `github.com/Tencent/WeKnora` | **1201 个文件** | 改了要动 `go.mod` + 每个 import，且**每次上游合并全文件冲突** |
| 环境变量名 `WEKNORA_*` | 77 个 | 与 `.env` 一一对应，漏一个就是启动失败 |
| Docker 镜像 `wechatopenai/weknora-*` | 5 处 | 上游发布的镜像，改了拉不下来 |
| 数据库名 `WeKnora`、Redis 命名空间 | | 改了连不上现有数据 |
| **localStorage 键名**（`WeKnora_theme`、`weknora_refresh_token` 等 15 个） | | 改了所有用户**掉登录、丢设置** |
| i18n 里的 WeKnora Cloud 相关文案、日语语言包 | | Cloud 功能已整体隐藏，文案看不到；日语暂无用户 |
| `weknora-widget.js` 的全局对象名 `WeKnora.init(...)` | | 已被 CRM 嵌入使用，改了要同步改调用方；外人只有开控制台才看得到 |
| 各连接器/搜索/Webhook 的 User-Agent、向量库集合描述 | | 只出现在对方服务器日志或数据库管理界面 |
| WeKnora Skill / Chrome 插件相关文案 | | 指向上游发布的外部产物，待定 |

---

## 四、升级流程

上游文件的改动**全部集中在一次提交里**，升级时：

```bash
git fetch upstream
git rebase upstream/main        # 冲突集中在上面第二节那 9 个文件
```

冲突时对照第二节逐条重新应用。`auth.go` 和 `Login.vue` 上游动得勤，
大概率每次都要处理，但都是一两行的改动，机械解决即可。

**新增的东西不会冲突**：`skills/preloaded/tyer-*`、`dev-app.ps1`、`dev-frontend.ps1`
都是新文件，rebase 时原样带过去。

---

## 五、我们自己加的东西要守的约定

上面几节讲的是"改上游文件"。这一节讲"加新东西"时怎么避免和上游撞车——
都是真撞过之后补上的。

### 数据库迁移走独立号段：migrations/teknowra/，永不与上游撞号

2026-09-14 起生效，替代先前"紧跟上游最大号往下排"的规则——那条规则每次
上游同步都要重演一遍"回滚→改号→重放"，三轮（85-88、89-92、93-95）之后
证明只要与上游共用一条号段，撞号就是必然，不是运气。

现行方案（一劳永逸）：

- 我们的迁移放 `migrations/teknowra/`，从 `000001` 自行编号；
- 水位线记在独立的 `teknowra_schema_migrations` 表（golang-migrate 的
  `x-migrations-table` 参数），与上游的 `schema_migrations` 互不相认；
- 启动时先跑上游全部迁移，再跑我们的（钩子在 container.go 调
  `database.RunTeKnowraMigrations`，实现和完整理由在
  `internal/database/migration_teknowra.go`）；
- 先上游后我们，所以我们的迁移可以引用上游的表，反向永远不行；
- SQLite 模式跳过我们的流（上游另有 migrations/sqlite 平行树，我们的
  功能从未做过 SQLite 变体）。

历史教训保留如下，提醒后人为什么共用号段的两个方向都是死路：


### `.gitignore` 的补充规则写在文件末尾

Windows 上 `go build ./cmd/server/` 产出 `server.exe`，上游（Linux）的规则
覆盖不到，`git add -A` 会把 400MB 的二进制提交进去，push 时被 GitHub 的
GH001 挡下。

这条规则**加过两次**——第一次加在特性分支上，合并时被上游的 `.gitignore`
覆盖没了。所以：**补充规则加在文件末尾并注明原因**，避开上游常改的区域，
且要加在基线分支而不是特性分支。

### 不要对包含上游提交的范围跑 `filter-branch`

同一天还踩过一次：清理误提交的二进制时用了 `filter-branch ... origin/$b..HEAD`，
而 `origin/$b` 停在合并之前，于是**上游那 149 个提交被一起重写成新 SHA**。

后果同样是无症状的——内容完全正确，编译、测试、推送全过，`diff` 是空的。
但共同祖先退回到了合并前，**下次合上游会把那 149 个提交整个重放一遍**。

要清理误提交的大文件，正确做法是：回到引入它的那个提交之前重做，
而不是对一段包含上游历史的范围做重写。
