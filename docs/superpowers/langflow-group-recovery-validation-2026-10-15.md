# Langflow 成组故障恢复：实现与验收

本轮针对上一轮实际故障：app 异常重启后，容器 ID 不变但网络命名空间改变，LF 仍在旧命名空间。保留共享回环拓扑，增加部署宿主侧恢复器，不修改企业用户权限、OAuth 或 Agent 业务协议；未提交、未外部发布、未上线。

## 恢复行为

`integrations/langflow/delivery/recovery.py` 仅监控显式指定的 Compose project。先校验活动版本、固定 app/LF 镜像、生成配置、外置环境与 Linux profile，再检查实际进程网络命名空间。app 由 Docker 自主异常重启；恢复器等待其 healthy 后，只从当前冻结清单重建 LF，禁止拉取或构建镜像。LF 的 namespace、health_check 和 app 代际检查通过后才报告 recovered。

监控有 OS 单实例锁、Docker 命令超时和30–300秒失败退避。LF 正常慢启动只等待，不重复重建。已停止/暂停的任一服务不自动启动。维护须先停宿主监控，再操作 Compose/版本切换；并发人工操作与恢复之间没有原子事务保证。配置内容变化后拒绝恢复，须由运维完成维护后重新启动监控。

恢复不保证崩溃时已经执行的任务续跑，也不自动重放工具副作用；本轮证明恢复后的新任务可以执行。恢复器是宿主管理进程，Docker 权限不提供给 LF、企业用户或模型工具。

## 验证方法与结果

所有故障仅注入本轮隔离 project `teknowra-delivery-linux-20261015`。先确认容器标签、固定镜像和 PID1=WeKnora，再在容器内向 PID1 发送 SIGABRT，验证 Docker 自动重启，而不是通过 docker stop/kill 模拟。生产与原本机业务后端不参与故障注入。

原型与冻结包各验证两次进程崩溃。每轮先无监控复现 namespace 失配，再让监控自动修复；随后在监控持续运行时再次崩溃，验证无需人工介入的自动恢复。两次恢复后均运行真实 Go Engine → 正式 Linux LF → 宿主合成模型，新任务成功且模型只调用一次。没有调用外部模型供应商、CRM 或企业知识。

人工停止 LF 后保持监控运行8秒，容器保持停止，随后停止监控再人工恢复 LF。冻结包直接从已安装 payload 启动，未在清单内生成 pycache；`--once` 返回 ready。平台原有登录、普通 runner 和私有图隔离在故障后复核，schema 仍114/clean。

- Linux LF 镜像内全套39项 Python 测试通过，包括13项恢复测试和旧包校验兼容测试。
- 真实 app 容器内14项 Go 运行时回归通过。
- 新包 I → J → I 成组切换、回退及每次新任务验证按证据记录；这仍是组件包回退，使用同一 app/LF 镜像，不冒称跨二进制或跨 schema 兼容性。

## 审查与修正

一次 fresh 只读审查发现两项 Important，均先编写用例复现，再修正并通过：

1. env 文件路径不变、内容变化可能在重建时被悄悄注入。现对两份 env 仅在内存计算摘要，并持续核对 profile；不输出/持久保存环境内容或其摘要。
2. 可写版本目录启动时导入 release 可能生成 pycache，触发严格清单拒绝。入口导入前禁用字节码，systemd 模板也使用 `python3 -B`；实际安装包入口另行复验。

真实 Docker 适配还修正了 LF 没有 Health 字段时的模板读取方式，使用 index 访问可选键，不将无 healthcheck 的 LF 判为配置损坏。

## 交付形式与证据

新候选 I/J 包含恢复器、Linux systemd 模板和维护说明；不覆盖历史包。旧完整白名单包仍可校验，两个恢复文件只能一起出现。Linux 服务模板需要替换版本和路径、单独配置宿主账号及锁文件权限；本轮在 Windows 宿主通过 Docker 进行故障验证，没有安装生产服务，不能声称已验证 Linux 开机自启。

研究目录：`D:/TeKnowra/research/selection/langflow-delivery-test-20261015`。

- `linux-isolated/group-recovery-evidence-final.json`：冻结包两次崩溃及人工停机证据。
- `linux-isolated/phase-recovery-*.json`：故障前后与版本切换的身份、镜像和实际 namespace 证据。
- `logs/recovery-monitor-final.log`：最终恢复状态事件，不含 Docker stderr 或令牌。
- `logs/recovery-review-fixes-red.log` / `-green.log`：两项审查问题的先失败后成功记录。
- `logs/recovery-final-python-tests.log`、`logs/recovery-final-runtime-suite.log`：最终测试。
- `group-recovery-delivery-evidence.json`：本轮最终摘要与工件摘要，以实际生成结果为准。

后续交付重点转为跨机受控镜像分发/可复现构建、对应前端与企业业务 Linux 验收，以及状态备份恢复。不得把本轮合成新任务成功替代这些验收。
