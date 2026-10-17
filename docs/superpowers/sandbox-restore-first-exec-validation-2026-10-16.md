# 恢复后首次技能任务修复与验证

本轮问题已修复，并完成针对性恢复验证。未提交、推送或上线；原有未提交修改保留。新镜像只作为本机隔离验证候选，尚未替换旧离线交付包。

## 根因

Docker Connect 在连接持久化旧绑定后先触发后台空闲回收，活动标记要等后续Exec才刷新。恢复后的沙箱可能已经运行、闲置数小时，在目录准备之前被这个回收流程删除。上轮日志同秒出现No such container与sweep reclaimed，模型下一次工具调用才恢复。上轮“活动标记不得早于容器启动时间”的修复保护新建容器，不能给已经运行很久的旧容器续活。

确定性回归在旧实现观察到：重连仍被回收、shell/script目录准备直接失败。修复后通过。仅修改产品Docker重连与会话执行准备两处代码；没有改权限、OAuth或业务逻辑。

## 修复约束

- Connect先实际刷新活动标记、检查成功，再触发回收；已消失的容器以NotFound交给原生命周期处理。
- 仅Docker普通执行，在工作目录准备明确NotFound/Terminal且业务命令尚未开始时，最多重新Resolve一次。租户/会话存在性、绑定锁及CAS沿用原实现。
- 权限、超时、服务不可用、未知错误均保留绑定；固定ExpectedSandboxID的维护与AsRoot安装操作不重建。
- 业务命令执行在恢复循环之外，不自动重放；若换了handle，丢弃旧容器输出快照。

## 验证结果

1. 新增单测先RED后GREEN。Windows沙箱包（排除一项仅POSIX测试）797个测试/子测试通过，2项原有跳过。被排除项TestWorkspaceBootstrapDoesNotRemoveOrMoveFiles依赖/bin/sh；不冒充Windows全套无排除通过。Linux完整沙箱包通过，首次测试构建缺start-desktop.sh夹具，补齐后全包通过。
2. 真实Docker两项通过：旧绑定活动标记续期；在目录准备边界实际删除本轮专属测试沙箱，第一次业务调用自动重新绑定并运行合同技能脚本，输出60000.00，实际业务命令计数严格1。测试禁用全局sweep，仅清理自己创建的测试会话。
3. 从上一轮原始备份恢复到新目录restored-16、新PostgreSQL卷/库、新Compose project teknowra-restorefix-20261016。所有备份摘要核验，schema114clean，12表业务数据匹配，验收后仅messages新增2条；候选app镜像为sha256:c97df55130d5b31383279c5114598fe3326452a79fced0f1deec447abbf322d2，其余7镜像不变，editor/runner身份保留。
4. sales01在正式前端恢复原会话2c90ef32-f722-4eb0-8a34-fd725b16b381，仅发送一次恢复后的新任务。第三轮assistant成功，trace bc240bac04a945b0a644c74d6760773c为ok，仅一个成功execute_skill_script工具调用，真实输出60000.00，exit0，保留代号LF-DELIVERY-1015-X9。没有模型失败后重试、没有第二次发送。
5. 本次实际续用了旧沙箱e8816c7e4a4d：创建于2026-10-15 14:05 UTC，旧活动时间14:13 UTC；首次新任务后仍为同ID，活动时间更新到10月16日。证明长期闲置旧绑定重连的实际路径已覆盖。

一次独立只读审查未发现Critical/Important；提出的Docker失败分类、维护/安装不替换与输出快照覆盖缺口已补测试并通过。补充边界测试在Windows执行；候选镜像包含其构建时的Linux整包与两项真实Docker测试，产品两处文件SHA与最终工作区逐一相同。没有伪称镜像内包含随后新增的所有测试源码。

## 恢复夹具发现的另一个问题

研究恢复器用with sqlite3.connect做完整性校验，context manager只结束事务，不关闭连接。Windows宿主仍持有WAL数据库句柄，Linux挂载启动时出现disk I/O error。终止这个研究校验进程、释放句柄后，Langflow正常启动200，之后第一次业务任务成功。已给本轮恢复器及上一轮通用研究恢复器加显式close；未改Langflow产品代码、未删除SQLite或业务数据。冷恢复首次服务启动存在这一研究夹具错误，不能隐去。

## 范围与交接

本轮验证是同机Docker上的隔离新卷恢复，不代替第二物理机、Linux systemd或跨应用/schema版本验证。本次修改未部署到原生8080服务；原服务只恢复本轮暂停的Langflow7861。测试8服务已stop，原备份、新旧数据库卷及失败记录全部保留。原服务最终健康见研究目录final-environment-evidence.json。

工件：D:/TeKnowra/research/selection/sandbox-restore-fix-20261016/ 下restore-evidence.json、final-source-evidence.json、real-docker-tests.log、windows-sandbox-tests.jsonl、first-restored-skill-success.png。带凭据的restored-16保持受限ACL，不属于公共交付包。
