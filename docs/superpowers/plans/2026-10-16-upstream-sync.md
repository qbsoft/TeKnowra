# 上游同步与定制兼容

用户授权拉取并合并 Tencent/WeKnora 最新更新。按 executing-plans 在当前工作区执行，沿用此前用户直接实施、保留未提交修改的选择，不另建工作树。

目标固定 `e07a190219a98bbebd87f6ec60701682ef0df093`（108个新增提交，v0.8.3），原HEAD `b7c739b91d4c9e5fb0d13572397b47407f3cfe97`，实际分支 `cursor/mcp-oauth-return-to-current-page`。保留分支，不推送、不上线、不混入原有未提交功能。

- [x] 抓取、核实目标和本地状态；51个修改文件、248个未跟踪文件在仓库外保存原始副本、SHA清单和补丁。
- [x] 合并上游并解决8处已提交fork冲突，审查自动合并权限/品牌/Actions/许可告知；建立本地merge提交。
- [x] 用保留的stash apply恢复未提交工作，解决兼容冲突，不删除stash或外部备份。
- [x] 定向回归、类型检查/构建、权限及原Agent集成兼容验证，依赖升级顺序和数据库迁移单独核实，不直接用新程序改原开发库。
- [x] 独立最终审查、证据及交接更新，明确实际验收范围；保留未提交修改。

备份：`D:/TeKnowra/research/upstream-sync-20261016/`，含before-merge.json、working-files、before.patch、stash-id.txt。备份分支见清单。未归档env、凭据、日志、数据库或忽略文件。

Ruling：先在清洁的已提交基线上合并并提交上游，再恢复未提交定制；不将Langflow/Eino等本机增量混入merge提交。原程序和冻结交付包保留。新迁移115—122不在原库上自动运行，先评估隔离验收。


进度（2026-10-16）：纯上游merge为 f0702b86f，目标e07a19021已是HEAD祖先。stash以具体SHA apply，未pop/drop；原299文件均在，248个未跟踪文件曾逐字节恢复（87个仅换行变化已恢复原bytes）。后续为兼容SDK/HITL，原OAuth resume测试和授权落地页有明确适配，最终清单另记。

Ruling：上游新增AgentEngine原生挂起接口。Eino、正式Langflow和旧Langflow验证器仍走原有有界in-process OAuth，不伪造native snapshot；EnableSuspension/SetInteractive无操作，Resume明确拒绝，避免重新执行已发生的业务调用。本轮不扩展跨进程恢复能力。

Ruling：上游assistant操作工具的excerpt与fork cronjob同包函数重名；只将fork工具私有帮助函数改名为cronjobExcerpt，保留两者语义。

审查发现共享OAuth提示owner租户已变caller，而handler仍用source租户resolve。已新增真实Gate/内存OAuth仓储的授权、取消、跳过回归；三分支均复现workspace mismatch，再最小修复提示resolve租户（令牌仍source+本人）。RED日志shared-oauth-red-4.local.log；GREEN日志shared-oauth-green.local.log已通过，包含原生prompt和OAuth handler定向回归。

验证：前端类型检查已通过；完整257文件测试1764项，1762通过、2跳过、0失败。修复了Windows换行导致的新上游SkillSettings源码断言误报，以及授权落地页字号未用现有设计变量的样式守卫问题；没有放宽阈值。浏览器只读确认原Agent列表仍在、新建执行引擎可选Langflow，未保存空表单；这使用原运行后端，不作为新后端/迁移验收证据。


后端回归进展：Eino/Langflow runtime全包、MCP客户端全包均通过；服务、会话handler、配置、类型、沙箱、审批、HITL七包按受影响能力定向回归通过。工具包MCP/Cron/AskUser/Operation定向回归最终通过（go-tools-green.local.log）；中间已适配新SDK测试夹具的四返回值OAuth调用，并修复原有Windows file URI测试夹具，使“禁止外部资源加载”断言真实执行，没有跳过或放宽安全校验。

本地提交：f0702b86f为纯merge；b425251f4仅提交本轮权限/编辑器/品牌/测试适配；aac54535a仅提交跨平台安全测试URI修正。后两次提交都没有包含原有dirty文件的完整功能增量；AgentEditorModal仅通过独立index blob提交MCP的unmet:false一行，页面Langflow功能继续未提交。

独立审查：只读审查发现并复核了共享OAuth caller/source边界修复；复核后无新增P1/P2。可选的legacy Gin-only caller回退专门用例未单独补充，不作为本轮阻塞项。

当前仅剩原生后端构建与前端产物构建。前端首次使用2304MB堆限额构建时OOM（exit134），不修改产品逻辑、构建优化或测试阈值；等待后端构建释放资源后提高该构建进程堆限额串行重试。冻结交付包、开发数据库与运行后端均未切换。


## 最终结果（2026-10-16 23:21，本机源码同步完成）

- 最终HEAD：`aac54535a32710e9913822ff1f0bd7c03937776b`，上游`e07a190219a98bbebd87f6ec60701682ef0df093`已是祖先。没有未解决冲突、MERGE_HEAD或暂存变更；原有功能继续保持未提交。
- 前端257个测试文件：1764项，1762通过、2跳过、0失败，exit0。跳过项为浏览器人工接管专用验收和本机没有POSIX shell的字面参数验收。类型检查exit0。
- 前端常规生产构建最终在3584MB堆限额下通过（exit0，1分22秒），输出仅在research校验目录；保留原2304MB OOM日志说明过程，没有修改构建功能或放宽检查。存在常规大chunk提示，不是构建失败。
- 后端Eino/Langflow runtime、MCP客户端测试通过；服务、会话、配置、类型、沙箱、审批、HITL七包定向回归通过；MCP/Cron/AskUser/Operation工具回归最终通过；共享OAuth与原生prompt handler回归通过。
- 后端完整`go build -p 1 -ldflags='-s -w' ./cmd/server`通过，校验exe SHA256为`16135c277bbda251df044dad471e34a28de627d7395dd70adbe0c2e99e8f5e0c`。编译时VCS记录为b425251f4、modified=true；随后唯一新增commit aac54535a只改测试URI，不改业务源。该exe包含本机既有未提交功能，不能仅凭HEAD重建或当作正式交付包。
- 原299文件无缺失。原248未跟踪文件中246个与原备份逐字节一致；另2个仅作明确适配：McpOAuthDone.vue改用现有字号变量、mcp_oauth_resume_test.go适配新Gate.Resolve参数。原stash和备份分支均保留；具体清单见仓库外final-state.json。
- 上游无新增workflow，仅app/helm修改；helm的Tencent仓库guard保留，本轮未push、不触发远端CI或发布。新LICENSE许可正文及第三方清单保留，补回fork额外通知文件引用。

范围边界：这是源码合并、静态/单元/定向集成与构建验证。没有启动合并后的新后端，没有在原开发库执行新迁移，没有用新后端重新做真实企业全链路，也没有重新封装或修改M/N、推送或上线。原生新助手/HITL等上游新产品能力需在后续隔离升级后验收；不要把旧后端下的前端入口可见视作新版本实机验收。

证据目录：`D:/TeKnowra/research/upstream-sync-20261016/`。主要日志为frontend-all、frontend-typecheck、frontend-build-final、go-runtime-tests（其中工具首次编译错误已由后续最终回归覆盖）、go-integration（其中工具首次编译错误已由后续最终回归覆盖）、go-tools-green、shared-oauth-green、backend-build。日志均为local文件，不提交。后端exe和frontend-dist只是本轮校验产物，未安装。

收尾约定：用户已明确授权在当前工作区合并上游并保留原修改。本轮没有新建待集成的功能分支，因此保持当前分支，不追加推送/PR菜单，不清理工作区，也不把已通过的风险定向验证扩展成不相关的全仓重跑。原stash、外部before快照、after-merge.patch及after-additions均保留。
