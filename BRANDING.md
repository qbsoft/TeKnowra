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

### WeKnora Cloud 为什么要三处一起堵

它不只是个设置页，还是**模型供应商**和**解析引擎**的一个选项。只藏设置页的话，
用户还能从「模型配置 → 新增模型 → 供应商」里选到它，然后去申请一个本产品
根本没有的云凭证。三个入口：

1. `Settings.vue` —— 左侧导航项
2. `ModelEditorDialog.vue` —— 模型供应商下拉
3. `ParserEngineSettings.vue` —— 解析引擎列表

---

## 三、刻意**没有**改的

| 类别 | 量 | 不改的原因 |
|---|---|---|
| Go 模块路径 `github.com/Tencent/WeKnora` | **1201 个文件** | 改了要动 `go.mod` + 每个 import，且**每次上游合并全文件冲突** |
| 环境变量名 `WEKNORA_*` | 77 个 | 与 `.env` 一一对应，漏一个就是启动失败 |
| Docker 镜像 `wechatopenai/weknora-*` | 5 处 | 上游发布的镜像，改了拉不下来 |
| 数据库名 `WeKnora`、Redis 命名空间 | | 改了连不上现有数据 |
| **localStorage 键名**（`WeKnora_theme`、`weknora_refresh_token` 等 15 个） | | 改了所有用户**掉登录、丢设置** |
| i18n 四个语言包（各 42 处） | 168 处 | 按产品决定保持原样 |
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

### 数据库迁移一律用 `000900` 起

**不要接着上游的号往下加。** 2026-08-20 合并上游时踩过：

    我们   000080_agent_cron_jobs
    上游   000080_knowledge_base_auto_tag_config

golang-migrate 以版本号为键，同号两份它只认一个（按文件名排序取到了我们那个），
**另一个被静默跳过**，然后把 `schema_migrations` 标成 version=84、dirty=f。

危险在于它**没有任何症状**：迁移状态显示一切正常，实际少了上游那一列，
要等某个依赖它的功能在运行时以莫名其妙的方式失败才会暴露。

所以：

- 我们的迁移从 `000900` 开始编号，上游短期内到不了那里
- 加之前先 `ls migrations/versioned/ | grep 0009` 确认没占用
- 迁移里一律用 `IF NOT EXISTS` / `IF EXISTS`，让重复应用是安全的

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
