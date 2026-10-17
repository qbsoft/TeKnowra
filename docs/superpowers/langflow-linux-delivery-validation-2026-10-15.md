# Langflow Linux 交付候选包：本机隔离验收

本轮范围：在本机 Docker 串行构建当前工作区后端、核对 Linux 依赖、生成不可覆盖的集成包，并验证隔离栈安装、运行及组件包回退。未提交、未推送、未发布外部镜像、未上线。不是企业业务全栈验收完成声明。

**后续更新：** 本文保留首轮 G/H 构建验收事实。后续成组异常恢复已实现并在冻结 I 包两次真实崩溃中通过，当前候选改为 I/J；详见同目录 `langflow-group-recovery-validation-2026-10-15.md`。下文“尚未实现恢复器”等描述仅是首轮历史状态，正式当前状态以新文档和 `group-recovery-delivery-evidence.json` 为准。

## 拓扑与运行约束

当前候选采用单实例 app/Langflow 共享网络命名空间。app 发布回环端口，Langflow 通过 `network_mode: service:app` 加入；短时能力桥仍仅监听 127.0.0.1。Langflow 不挂载平台环境、数据库/OAuth 凭据、服务凭据或 Docker socket。编辑身份、运行身份均为独立普通用户，私有图隔离。

首次安装顺序必须为：基础设施完成初始化 → app 健康检查通过 → Langflow 启动 → 两端健康、真实命名空间及一次执行验证。平台健康检查不依赖 Langflow，以免启动循环。Compose 模板已经增加 app `/health` 检查及 Langflow 的 `service_healthy` 依赖；本轮切换脚本还显式等待 app 健康。

**共享网络生命周期限制尚未消除。** 实测 app 独立自动重启会让 Langflow 留在旧网络命名空间，尽管配置仍指向同一个 app 容器 ID。`depends_on` 只控制启动顺序，不能把运行中的异常重启变成成组恢复。当前只能按成组停止、成组重建方式人工恢复；正式无人值守运行还需补齐成组故障恢复与验证，或调整网络承载方式。不能凭 Docker restart 策略宣称高可用。

## 可核验工件

研究根：`D:/TeKnowra/research/selection/langflow-delivery-test-20261015`。

| 工件 | 固定标识 |
|---|---|
| LF amd64 镜像 | `langflowai/langflow@sha256:e5540a56a38ab34fcff604996c5d7797b88d16e444bd7ceddead077c5e277ac6` |
| 当前工作区 app 镜像 | `localhost:15000/teknowra/app@sha256:f0ad22a7a774b424873509658dba1601bddc0fc31d972a6b625e8b09eb6e3791` |
| app 实际 config ID | `sha256:c226f1cdbe7143c360c0fbb5d9914e334f6e6f905fd480e9cbcdb8283fa9b528` |
| 后端二进制 SHA256 | `af5341cd726dd6eb1112f97e88c9c8091b47a606e31e2cf0054191169070f8b4` |
| 集成包 | `artifacts/linux-container-20261015-g.zip`、`linux-container-20261015-h.zip` |
| Linux 依赖锁/描述 | `linux-runtime-lock.json`、`linux-profile/`，Python 3.14.7、441 项包元数据及 3 项关键源码 |

app 包含本轮真正构建的 Anydoc 和 BrowserSkill 0.3.1。Go 编译限制并行度为 1，Rust jobs=1，重型阶段串行。旧产品镜像的 BrowserSkill 为 0.2.1，未用其替代当前构建。实际 Go 1.26.9；二进制 BuildTime 仍 unknown，构建追溯信息仍需正式化。

本机 registry 仅监听 127.0.0.1:15000。上述镜像引用用于本机验收，不能在另一台机器直接拉取。跨机交付前必须确定受控镜像分发渠道或离线导入方案，再据目标不可变引用重新冻结和验收包。ZIP 包是组件、依赖指纹和镜像声明，**不包含全部容器镜像**。

## 本轮故障与更正

最初独立构建脚本根据文件名排除 credential/token/secret/backup，误删了原有的 000035_add_credentials 和 000054_invitation_tokens 两组 SQL。数据库迁移可以跳过缺失的版本文件继续到 114，因此仅看最终迁移版本无法证明工件完整，注册最终报缺列。

这不是产品原始源码缺迁移。临时新增的 115 迁移源文件已撤掉；A–F 过程包和旧 schema115 隔离数据库只作故障证据，禁止作为交付候选。修正脚本允许合法 SQL，另从源码冻结完整 305 个 SQL 并逐项比较镜像内 SHA。补全层基于未含临时 115 的原 app，未重复编译 Rust/Go，也未更改后端二进制。

使用新建 `delivery-pgdata-complete` 隔离卷从零完成原始迁移，版本 114、dirty=false；旧测试数据库卷保留，未执行降级或删除。新注册、登录、当前身份读取分别 HTTP 201/200/200；显式测试 DSN 下，真实 GORM Tenant 创建及 nullable JSONB 凭据写读通过。测试业务写入事务回滚，迁移本身持久化。

另一次启动失败来自新数据库初始化期间 app 连接拒绝、自动重启，导致共享命名空间失配。健康检查与启动等待修正后重新验收；上文运行中独立重启限制仍成立。

## 验收证据与范围

最终机器可读结果在研究根 `linux-build-evidence.json`，细项在 `linux-isolated/phase-*.json`、`execution-*.json`、`complete-migrations-evidence.json` 与 `logs/linux-final-*.log`。

- 固定正式 Linux LF 镜像中组件、技能、交付测试共 25 项。
- 实际 app 容器内运行 Go 执行器测试；真实 Go Engine → LF → 宿主合成模型调用，外部模型供应商、企业知识库、CRM 均不调用。
- G → H → G 需逐阶段保留健康、镜像 ID、网络命名空间、已有登录、普通运行身份与私有图拒绝证据，并逐次执行真实链路。G/H 仅组件包注释标记不同，app/LF 镜像相同；这证明组件包切换和成组重建，不能证明不同二进制、不同 LF 依赖或数据库 schema 的升级兼容性。
- PostgreSQL 新库原始全迁移与租户凭据回归，不代表所有数据库集成测试已跑完。

## 使用及回退

`integrations/langflow/delivery/README.md` 说明通用 verify/install/activate/container-config 命令。先 verify 包，再 install 到新目录；在匹配的 Linux 镜像实际捕获依赖并 activate，不能拿 Windows profile 或人工构造 runtime 对象替代。绑定、服务凭据、状态均在版本目录外，由部署方限制权限。激活清单不热替换运行进程。

本机验收脚本 `Switch-LinuxComplete.ps1` 明确限定本轮 project、G/H 版本和 D 盘路径，不是通用生产安装器。它先停 app/LF，核验激活和生成配置，再启动 app 并等待健康，最后启动 LF。启动失败必须停止交付并保留日志，不能仅改 current.json 后声明切换成功。需额外核验实际 `/proc/.../ns/net` 相同，不能只比较 Docker 配置。

回退同样成组重建，不自动降级数据库。业务状态备份与恢复需另行演练。禁止 `down -v`、全局 Docker 清理、删除数据库或 OAuth 卷。

## 正式交付前仍需补齐

1. 成组异常恢复机制及故障后新任务/进行中任务的明确收敛行为；已有启动顺序约束不能替代它。
2. 受控跨机镜像分发与可复现构建入口，消除研究机绝对路径和 unknown 构建时间；按目标引用重新冻结。
3. 与当前后端对应的前端工件，以及浏览器经代理的真实企业业务验收。本轮没有新建完整前端容器，合成链路不能替代知识、技能、CRM、OAuth 的 Linux 全流程。
4. 状态备份恢复，以及不同应用/依赖版本升级和回退的兼容性验证。容量指标按实际交付规模另测，不能由单实例合成请求推断。

本机分批构建可行；本轮候选不等于已具备正式无人值守交付条件。
