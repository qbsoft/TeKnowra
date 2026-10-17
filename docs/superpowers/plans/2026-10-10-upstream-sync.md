# 上游完整同步计划

> **For agentic workers:** Use superpowers:executing-plans 执行，保留用户已有未提交工作。

**Goal:** 将 Tencent/WeKnora 最新 main 全部合入本机 TeKnowra，修复兼容问题。
**Architecture:** 本地备份与 Git stash 保存当前工作；合并上游和已提交 fork 差异后恢复未提交工作，再适配接口变化。
**Tech Stack:** Git、Go、Vue、TypeScript、PowerShell。
**Spec:** 用户授权“都拉下来吧”；AGENTS.md 与 BRANDING.md。

## Global Constraints
- 不推送、不部署；不提交凭据或原有未提交工作。
- 保留按人 OAuth、工具白名单、共享权限、品牌、定时任务、CRM 业务上下文、Eino、追问去重和口径折叠。
- 不启动新增 Actions；不删除数据库或模拟数据。

## Review Focus
- 同一浏览器换人不得继承授权；共享智能体必须复核来源空间。
- MCP 服务和工具白名单发现与调用两头校验。
- 模型协议重构后的 Eino 接口兼容。
- 上游迁移与 migrations/teknowra 独立号段。
- 更新扩展/浏览器配套组件的需要与本机验收范围。

## Tasks
- [x] 1. 备份当前 HEAD 和修改文件，抓取上游，记录目标 SHA。
- [x] 2. 保存工作区，合并上游，按 BRANDING.md 处理 fork 冲突和 CI 差异。
- [x] 3. 验证合并代码，创建本地 merge commit，恢复原有未提交工作并适配。
- [x] 4. 安装更新的依赖，运行前端类型检查/构建/测试、后端测试和构建。
- [x] 5. 本机启动与浏览器验证，记录实际通过与仍需处理事项。

## Ledger
- Ruling: 在原目录备份后执行，用户要求完整同步并持续完成；原未提交工作继续保持未提交。
- 上游目标：097bd475f601d48a82ed7aaa2f9676dd82f15d97，2026-10-10 main；相对旧同步基线新增 408 个提交。已验证目标为 HEAD 的祖先。
- 合并提交：16d3adce8；纯前端兼容提交：103a1c461。没有推送或生产部署。继续使用当前原分支，不擅自切换 main。
- 原工作备份：backup/pre-upstream-20261010、stash af844058d06d409918f5f54e443f99bc0e03189a、仓库外 research/upstream-sync-20261010/working-files；原有未提交工作已恢复并保持未提交，stash 未删除。随上游删除的旧模型文件，单次 ThinkingControl 与连接重放适配已迁入新 api 层，原文件仍有备份。
- 冲突兼容：模型协议重构、search_knowledge 单 query、新引擎 checkpoint 接口、共享来源空间/按人 MCP OAuth、技能工具依赖提醒、cron 防递归、追问去重、口径折叠。
- 新增 helm Action 仅允许 Tencent/WeKnora 运行；fork 独立迁移目录和上线脚本保留。
- 前端：Node 24.19.0，npm ci 成功；类型检查通过；1501 项测试中 1500 通过、0 失败、1 项已有 human handoff 浏览器测试跳过。POSIX 文字安全测试用本机 Git Bash 实际通过。
- 构建：后端 Windows 原生构建成功；前端生产构建成功（NODE_OPTIONS=--max-old-space-size=4096）。初次并行构建遇内存不足，改为串行、扩大 Node 堆后完成；本机启动脚本已验证会选择已有兼容 Node 运行时。
- Go 验证：internal/models/api/...、internal/agent/einoruntime 全包通过；service、handler、session、tools、config 的 OAuth/来源空间/工具授权/cron/Eino/业务上下文/续问针对性回归通过。
- 验证限制：首次扩展 Go 测试暴露上游测试对 Windows 路径、/bin/sh、沙箱包安装环境的假设，未声称完整 Go 全套通过。例如 MCP schema 测试构造反斜杠 file URI，校验器先拒绝非法 URI，因此未命中其预期报错文本。Windows 宿主目录/沙箱新能力未逐项验收；浏览器扩展配套安装与上游全部新增功能未全面验证。
- 数据：本机平台 PostgreSQL 已生成并核验可读取的 pg_dump 备份（仓库外保存，不提交）；新版 schema_migrations=114、dirty=false。未删除平台或 CRM 数据、模拟数据，不操作其他项目容器。
- 本机：后端 8080、前端 5173 已启动，health=ok，前端代理统一登录配置成功。复用浏览器已有本机 Casdoor admin 会话，完整登录回到知识库，原 2 个知识库、12 个智能体均可显示。
- 浏览器经营问数 V2：会话 5fbe41fa-780d-45f7-968b-068236e209c0；查询 2026-08 销售额最高业务员返回张三、371800 元、2 单，点击继续问下钻返回 SQ202608060009（222600）与 SQ202608060010（149200）；口径说明展开/收起正常，推荐问题来自当前结果。截图 browser-v2-drilldown.png 保存于仓库外验证目录。
- 浏览器 Eino 合同审查 V2：会话 cfb4b9fc-8004-4d6b-8845-3aaa2d1c2419；TEST-A-001 返回金额 100000 元、预付款 40%、财务审批与担保风险；引用打开《V2 合成合同与审查政策》来源侧栏，未执行 CRM 查询或写入。截图 browser-eino-citation.png 保存于仓库外验证目录。
- 独立只读审查：未发现有证据的新增 Critical/Important 问题；审查覆盖协议迁移、检索结果 ID、OAuth/来源空间、cron、迁移和新增 Actions。未将静态审查当作公网/全部功能验收。
- 最终 HEAD b7c739b91（本机兼容 Node 选择）；未解决冲突 0，暂存区为空。剩余 49 条工作区记录包含原有工作及其兼容适配，不混入本轮同步提交；health=ok、前端 HTTP=200。仅本机更新，生产版本未改变。
