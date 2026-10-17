# 恢复后首次技能执行修复计划

> 当前工作区直接实施，保留已有未提交修改；使用 executing-plans、systematic-debugging、test-driven-development。不上线、不提交。

**目标：** 恢复旧会话后，空闲沙箱重连与回收竞争不再让尚未启动的首次技能命令失败；不自动重放已经启动的业务命令。

**已核实链路：** Docker Connect 只检查/恢复状态，随后异步 sweep；活动时间直到后续 Exec 才刷新。上轮工作目录准备报 No such container，同秒 sweep reclaimed，模型第二次工具调用才成功。

**实施范围：** Docker 重连刷新活动时间后再触发回收；仅 Docker、非维护操作在目录准备证明沙箱已消失时重新解析一次。超时/权限/未知错误不替换；实际业务命令不重试。

- [x] 增加确定性回归，先观察旧实现失败。
- [x] 最小修复并运行 sandbox 包测试及安全边界测试。
- [x] 用真实 Docker 恢复旧绑定、过期活动标记及受控回收，验证首次技能命令只运行一次。
- [x] 在资源允许时隔离恢复环境做真实聊天验证；若无法完成，明确限制，不能用单测代替。
- [x] 一次独立审查、记录证据、恢复临时暂停的原服务，不上线。

Ruling：沿用用户此前选择的当前工作区，不建新 worktree，避免丢失既有未提交集成代码。内存不足时仅暂停本任务原 Langflow；编译缓存/日志置 D 盘，不做全局清理。


执行记录：新增旧实现失败回归后最小修复。独立审查0Critical/Important，2项Minor测试覆盖已补；Windows797pass/2skip（另排除POSIX-only1项），Linux完整沙箱包pass，两项真实Dockerpass；新目录/新PG卷恢复后sales01首次聊天一次调用脚本成功，旧沙箱ID保留。夹具SQLite宿主连接未关闭的问题另行查明并显式close，未变更产品SQLite。隔离8服务已stop，原7861已恢复200，原8080/5173/MCP元数据均200，final-environment-evidence已记录。

Ruling：不为补充测试再次构建大型候选镜像。最终两处产品源码与已构建上下文逐SHA相同；补充边界测试已在Windows跑完整包，镜像内Linux包与两项真实Docker测试按其构建快照留证。保留初次测试夹具失败，不夸大跨机/跨版本覆盖。
