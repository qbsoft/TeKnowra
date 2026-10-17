# Langflow 交付移交补齐计划

范围调整（2026-10-16）：用户明确决定不做干净Linux环境验收，该项不再作为本次交付阻塞。准备工作已完成，但不继续执行宿主Linux/systemd/空缓存验收，不将取消算作通过。保留封存M/N及现有修改，仍不上线。

> 使用 executing-plans 在当前工作区直接实施，保留全部未提交修改，不提交、不推送、不上线。用户已明确授权本轮顺序与范围，不再次请求方案审批。

**Goal:** 在不改动已封存M集合的前提下，提供可移交部署依赖、停写备份恢复工具、对应源码归档，并准备干净Linux验收。

**Architecture:** 新增与M集合配套的handoff包，工具仅Python标准库和Docker CLI。通过显式project与服务标签约束作用域，备份时要求写入服务已停止；恢复只写新目录和新数据库，不覆盖旧状态，不自动启动生产。源码从已冻结构建上下文、修复覆盖层、已安装M组件以及已核对前端源文件组合，区分构建时来源与当前维护代码。

**Tech Stack:** Python 3.12+、Docker Engine/Compose、PostgreSQL custom dump、ZIP/SHA256、Linux systemd模板。

**Spec:** 本轮用户要求及上一轮 `langflow-fixed-release-validation-2026-10-16.md` 的四项交付缺口；只准备干净Linux，不擅自启动新VM或上线。

## Global Constraints

- 所有大文件/临时目录在D；D保留至少8GiB，不做全局清理。
- 不改封存M包；源码和通用配套不含.env、凭据、日志或业务备份。私有备份另置受限目录。
- 备份要求显式停写；保持数据库角色/schema114，不支持跨schema自动降级。
- 恢复到不存在的新目录/新数据库，失败保留证据，不删除原卷/失败目标。
- 本轮不追求字节级可复现重编译；必须明确记录源码/镜像对应证据以及缺失项，不能把当前HEAD冒充构建来源。

## Review Focus

- ZIP路径穿越、重复成员、摘要损坏、symlink/reparse点必须在恢复写入前拒绝。
- project不符或写入服务仍在运行必须拒绝备份；数据库恢复禁止使用其他project容器。
- 已存在的恢复目录/数据库必须拒绝，不能覆盖现有状态。
- 企业密钥/SQLite目录只进入受限备份，CLI错误不输出Docker敏感stderr。
- 源码快照必须核对冻结指纹，不能仅打包当前dirty工作区声称完全匹配镜像。

## Tasks

- [x] 1. 补齐部署基线、依赖清单、无凭据模板和Linux验收步骤；静态核对现有安装器接口。
- [x] 2. 先写备份恢复边界测试并看到失败，实现 `delivery/backup_restore.py`；新增测试与既有交付回归通过。
- [x] 3. 对本机指定已停止M恢复栈做新备份、恢复到新目录/新PG数据库；摘要、SQLite、schema与行数核对，停止测试数据库。
- [x] 4. 归档可证明的完整构建源码/修复层/前端/组件，含指纹和构建说明；封存独立配套包与清单，不改M。
- [x] 5. 单次独立只读审查，修复确认问题；准备Linux前置条件/执行检查表，更新接管与验收记录。

验收说明：`../langflow-handoff-validation-2026-10-16.md`。独立配套N已封存；62项Windows测试、8项Linux容器备份工具测试通过，正式Linux宿主验收只准备未执行。

Task ledger：部署/恢复接口与原switch约束核对；前端独立overlay，不传入四镜像switch。新备份工具实际演练通过，所有相关PG停止。单次review确认RedisCompose插值P1，增加真实Compose配置回归先RED后GREEN，最终62项通过。源归档按实际builder+冻结覆盖层，不把HEAD冒充版本。原M摘要复核全部通过。

Ruling: 用户要求先准备干净Linux而非立即验收；本机资源有限，先只做资源探测和可移交步骤，不启动额外daemon/VM，不把Docker容器当成Linux宿主重启证据。

Ruling: 当前工作区与前序授权保持一致；不新建worktree、不提交，不覆盖已有dirty文件。M集合不可变，新增handoff集合独立封存。
