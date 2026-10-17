# 双引擎运行保障实施与验收

目标与授权：用户已批准本机运行中断收尾和小规模双引擎并发验证，发现问题最小修复。保留dirty工作区、所有旧消息和修改，不提交、不上线、不生成交付包。直接当前工作区执行，不另建工作树。

1. 中断：先补失败用例，复用Redis live-run CAS与现有中断消息收尾。单实例显式配置时，以进程实例标识识别新任务残留标记；新判断默认关闭，不替代原有sweep配置，本机继续保持sweep=false。新任务消息快照保存实际执行引擎/定义/进程归属；所有已接入Eino及Langflow均能收尾，已完成/仍活跃/等待人工输入的任务不改。不自动续跑、不重放工具，不扫描改写历史。
2. 并发：复用runbudget包，双引擎共享有界本机执行槽，满额立即安全拒绝，不排无限队列；取消、报错、超时均释放。每任务已有时间/调用次数预算不改。默认2槽（保守本机起点，不是生产容量承诺），OAuth等待占槽。
3. 验证：定向单测RED→GREEN；必要一次本机exe构建。用两个合成只读任务同时运行并记录资源；中断正在执行的两个任务，重启后通过页面确认失败可持久化、同会话可重新提交、原调用未重放。满额/取消/超时路径用受控定向测试，避免重复旧MCP登录和完整业务回归。只为本轮新测试会话追加数据；校验旧历史哈希、原资源服务及数据库状态不变。

当前事实：旧中断处理仅识别contract-review-v1和customer-credit-readonly-v1；本机启动关闭全前缀sweep，读取live标记会续TTL。模型并发governor只管后台，交互任务没有共享上限。修复不得通过开启全局清扫或修改事实审核绕过问题。

验收记录待追加。

## 实施记录

- RED：Redis新实例读取旧owner标记仍返回old-message；新增摘要、多专家和Langflow中断收尾均不触发。独立审查另发现消息先持久化、marker后发布的刷新竞态，新增“无owner且无首事件不能证明崩溃”用例已观察RED。
- 修复：新live标记有实例owner，单实例显式启用时GetLiveRun对上一owner使用完整原值CAS删除，无全前缀清理。新消息执行快照保存进程UUID，同进程消息不判中断，旧无owner无事件保守；普通KnowledgeQA清空嵌入引擎标记。中断只记安全失败、关闭旧消息，不自动重跑。
- 两引擎Execute共用runbudget有界槽，默认2/可配置1..32；达到上限立即结束为繁忙错误，不无限排队。作用域为单进程嵌入引擎，不宣称分布式租约、租户公平队列或生产容量。等待OAuth占槽。实际Execute入口满额不调用provider，错误释放由定向用例覆盖。
- 最终5包本轮定向测试exit0（stream、handler/session、runbudget、einoruntime、langflowruntime，-vet=off，不是整仓测试），覆盖新owner/legacy保留、旧定义/新定义收尾、同进程发布窗口保护、待输入保护、槽满额/幂等释放/取消/超时/实际引擎早退。旧8并发图隔离用例明确设8槽继续检查原隔离目标。独立只读审查发现的Important已修复复核，无新增Critical/Important。
- 本机程序构建中；真实中断与并发资源验收尚未进行，不能据此宣称完成。

## 最终验收与收口

- 构建exit0，最终exe SHA256 `84c2106173ed11bef80b8eb176faeacc86bb40f2efdd4d33c90be5b8b430f944`。仅本机8080替换为runtime-guards程序，正常入口复用原launcher，明确单实例true、并发2、sweep=false、禁迁移/临时模型日志。当前PID26392；原四个资源服务PID未变。
- 首次监控已观察两引擎同时活跃，但Windows路径比较使用斜杠不一致，保护断言阻止kill；首轮任务正常完成，未伪记为中断通过。修正验证脚本Path.resolve比较后，两引擎任务同时执行时终止PID6192，成功重启，Eino消息858b0dfb-df70-4a1f-b1e2-7539913f2c08、LF消息afc05c56-d3f6-46d3-a6d6-f5770c5515db均明确保存进程中断，未重放。
- 实机发现LF首次ContinueStream正文空白（DB内容已保存，仅有toast）。复用前端preserveEinoErrorDisplay，接受错误事件自身definition或engine，不依赖已有timeline；仍failed=true。新增整handler用例无图片fixture观察真实断言RED→GREEN，vue-tsc exit0；初版误复用图片恢复fixture的超时不计产品超时。只读审查再次无Critical/Important。后端源码未变，不重构建；仅LF消息d855f4c6-1103-41d9-8b04-c8af1bc32d58再中断PID37136后，首次恢复页面直接显示正文，通过，不再重复Eino项。
- 最终重启PID26392，同原两会话并发重提成功：Eino 6ee9efbb-e267-4a8e-90e9-911f5f57af24四步骤；LF 3a13713b-2444-4d49-b1d4-3b6a57b464dc直接利用已有授权会话上下文回答，0工具步骤，是执行并发证据，不伪称新增检索验收。新任务完成后Redis live标记均空。LF四次合法请求对应ok/error/error/ok四trace，Eino0 LF trace，无自动任务重放。
- 本机样本：两引擎活动区间重叠，后端主进程RSS峰值278.6 MiB、LF主进程508.4 MiB、整机可用内存最低1631.9 MiB。默认2槽为本机保守起点；不是生产容量/高负载/多实例结论。满额及取消/超时释放走受控定向用例，避免重复旧真实模型/业务回归。
- `verify_runtime_guards.local.py` exit0：39项源码及exe哈希一致、原1632条消息校验字段哈希不变、23个Agent配置不变、122/114均clean、MCP配置不变、原四服务PID不变。报告dual-runtime-validation.local.json，candidate real_acceptance_complete=true。旧LF摘要Agent原自动索引策略未改，新测试问答沿原策略有knowledge_id；Eino新任务无索引。没有批改旧历史/Agent或权限。
- 所有任务/失败证据保留，没有上线、提交、推送、交付包、镜像或源码归档。工具安全策略拒绝本轮中止构建临时目录清理，未提供具体原因；已停止清理，不绕过，保留该临时目录。
