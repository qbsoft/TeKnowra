# 正式前端、企业业务与备份恢复验收：部分通过，未上线

本轮没有上线、提交、推送或修改产品源码。以下是本机 Linux 容器、正式构建 SPA 与独立新卷恢复的实测证据。首次恢复后技能任务失败，尚不能判定整体验收通过。

## 已通过

- 当前前端重新生产构建，复用正式 nginx 配置和入口，API 代理返回正确 JSON；浏览器 admin/sales01 登录成功。
- 前端实际容器 ID：`sha256:e2db43ba58ba94f4e7f879118810a5b2aabf489f701c2fe33ef5cbfab7122167`。211 个静态文件与构建摘要匹配，885 个源文件另存摘要；Git HEAD 不冒充含未提交修改的实际工件身份。
- admin 在正式前端发布知识/技能草稿为 `Linux正式前端验收-V5`（`1266ed79-2970-48a0-aeba-2dec17265299`）。原草稿内的“共享验收-V4”是旧提示词文案，不是本次实际运行版本。
- sales01 在接收空间10001使用源空间10000的只读共享 Agent，真实 DeepSeek 调用、合成合同知识检索、SKILL.md/REFERENCE.md读取、report.py计算全部通过。首轮返回40000.00与来源，第二轮实际执行50%参数，返回50000.00并保留会话代号。数据库答案完成、LF两条trace均ok，工具分别4次/1次且全成功。
- 新独立数据库卷与文件目录；源本机开发数据库只读导出，未写来源，未连接生产环境。

证据目录：`D:/TeKnowra/research/selection/langflow-delivery-test-20261015/frontend-business/`。

- `frontend-evidence.json`、`frontend-source-manifest.json`、`frontend-manifest.json`
- `business-before-backup-evidence.json`、`shared-multi-turn.png`
- `backup-preflight.json`（仅预检）

正式前端镜像归档：`../artifacts/frontend-linux-20261015-a.tar.gz`，35,392,023字节，SHA256 `13a45874abbe899579ecd20d8a43276e3c7df9abbe2544fef783478991b2cdfa`，配套 manifest。未含凭据和业务状态，未上传；尚未并入旧四镜像离线B包。

## 实测补齐的环境配置

技能脚本需要显式启用 Docker 后端，且运行账号必须能访问 daemon。Docker Desktop 的 GID0 socket 与现有降权入口不兼容，本机使用只监听 Unix socket 的内部 relay，后端仍 UID1000，宿主 socket 不 chmod，Langflow不挂 socket。它具有完整 daemon 管理权限，仅作为本机验收适配器；不能声称已验证目标Linux宿主的部署配置。

技能包/知识原文件还依赖对象存储。本机测试桶约156KiB，复制到独立MinIO；为其配置精确主机白名单，不关闭整体SSRF防护。目录ACL须让授权用户与SYSTEM权限向子项继承。`LOG_FORMAT`是日志模板，填入字面量json会丢失真实诊断，应使用默认格式或正确模板。

前三条admin合同尝试和一条sales合同尝试在配置补齐前失败，失败记录保留，不能计为成功。最终成功会话为 `2c90ef32-f722-4eb0-8a34-fd725b16b381`。

## 备份与新卷恢复已执行

一致停写备份实际生成116个文件，归档总计278,094,334字节，脚本耗时64.58秒。包含PostgreSQL逻辑备份、Langflow SQLite/密钥、发布/绑定/服务身份、对象桶、文件与配置，以及本轮实际技能/前端/MinIO镜像；凭据备份仅当前用户与SYSTEM可读，不作为公共交付包。

备份SHA、ZIP逐文件SHA、SQLite完整性与真实镜像load通过。原测试栈停止并完整保留，新目录restored-01及新PG卷恢复。首次pg_isready连到initdb临时Unix socket，pg_restore被初始化正常fast shutdown打断；不是OOM。已将研究恢复器改为TCP就绪检查，保留失败库restored，另建restored_v2成功恢复。12张业务表行数相同、schema114clean、8容器实际镜像ID相同、普通editor/runner身份保留、env原字节不变。成功重试脚本64.31秒，不将其冒充含诊断时间的总体RTO。

恢复后浏览器保留sales01/admin登录、原聊天历史和Linux正式前端验收-V5发布。首次新任务trace e49f92c3790f4150a3b5217def3eb992失败：shell_exec启动前旧沙箱被空闲回收，日志No such container与同秒sweep reclaimed吻合；随后同轮工具重建后算出60000，但整轮按失败处理。未改产品源码、未删失败记录。独立新任务成功，实际脚本60000.00、exit0并保留会话代号；四轮trace依次ok/ok/error/ok。因此数据恢复与后续可运行已证实，首次恢复任务可靠性仍为交付缺口。精确并发交错及同daemon环境隔离归属仍需后续修复验证，不宣称已唯一归因到某一行代码。

证据：backup-evidence.json、restore-evidence.json、business-restored-evidence.json、restored-contract-new-task.png、restored-admin-release.png。所有数据备份保留在受限backup-01，不外发。

## CRM与待完成项

经正式UI重新发布Linux合同与信用-V2（83e04731-36a2-4acb-91a9-fd2e1bd0f1e2），sales01首轮会话00fb5c9a-fcdb-4416-be65-0cfd10339aa2真实知识检索、子Agent委派及三个CRM工具成功，服务端七条认证均sales01/dept4；customer_id全程字符串。该会话后续新OAuth尝试超时，首轮检查点不等于整会话全成功。

隔离client已改回调127.0.0.1:18173；仅修改测试副本，未撤销来源授权。MCP临时白名单只追加该回调，结束须恢复原进程配置。Windows本机issuer回环使用固定目标适配器，不作为正式Linux默认拓扑。

恢复后全新CRM会话d8ec7557-c4bd-46c0-947b-e12fa4e60b88在授权卡片暂停，assistant e91c78a1-b259-4c32-8396-5e353f71b41e；已打开新授权页并请用户以sales01登录，尚未认定同任务恢复通过。界面实际倒计时仍10分钟，不将草稿1800设置当运行生效证据。

结束前还需核对该次新OAuth同assistant恢复，停止本轮隔离栈并恢复原7861与MCP临时配置。当前隔离恢复8服务运行，源测试8服务停止，原7861暂停；不可按旧交接的端口状态行动。

## 验收边界

同机Docker daemon缓存、新目录/新卷与实际image load已验证；不是第二物理机/空daemon灾备，不包含外部CRM/MCP自身灾备、Linux宿主systemd开机自启或不同应用/schema跨版本兼容。正式前端独立工件尚未并入旧四镜像离线B。当前不建议宣布可正式交付，未上线。

## 23:42 本轮授权重试最终结果（覆盖上方待授权状态）

用户确认之前误填admin，两个错误身份会话931571fe-f918-40d1-9491-beec65e5a1b7、71797b40-25a8-4bfb-b824-801be8886d6a保留，不计sales通过。仅删隔离库token时缓存连接可再次刷新写入token，因此停止隔离app/LF、清除该隔离principal/service令牌，再按组重建app/LF/回环/前端以清缓存；未写原库、未撤销原服务授权。

新会话8aaeaf90-7d21-46f8-af08-e2126e42979c真实显示授权暂停，点击新授权页后用户立即完成sales01登录。原assistant 85da183a-74c5-4f07-b5c4-4867ac0ff118在同会话完成，trace eae5ef3f69174cba89561e331a8e491c为ok。山东佳航名称检索为0，未读取不可见客户信用。随后同会话独立查询东营宇创，find_customer→check_customer_access(readable=true)→get_customer_credit_control真实成功，客户ID2073967557021581313始终字符串。两条trace全部ok，时间窗口内10条服务端认证全部sales01/dept4，无admin混入。证据restored-fresh-oauth-evidence.json、restored-crm-sales01-resumed.png、restored-crm-sales01-complete.png。信用正文/令牌未复制进本说明。

本轮授权同任务恢复已通过；首次恢复后技能任务沙箱空闲回收竞态仍未修复，整体验收仍部分通过。结束已stop恢复8服务，源8服务原样停止，卷/失败库/备份全保留。MCP原回调白名单已恢复，原LF7861启动恢复中，健康以最终handoff记录为准。全程未上线、未提交、未推送。

最终环境核验：原7861/8080/5173/MCP元数据均200，CRM9999监听，MCP实际进程已恢复原回调白名单。两个本轮测试项目均无运行容器，全部卷/备份/失败库保留；final-environment-evidence.json为最终环境记录。


## 2026-10-16 缺口关闭续轮

恢复后首次技能任务被空闲回收打断的问题已修复。经新目录/新PG卷恢复，sales01首次聊天一次成功，真实脚本60000.00、一个工具调用、trace bc240bac04a945b0a644c74d6760773c为ok，原长期空闲沙箱ID保留。详见sandbox-restore-first-exec-validation-2026-10-16.md。本轮只关闭该缺口，不把它等同于正式交付包已更新或生产已上线。
