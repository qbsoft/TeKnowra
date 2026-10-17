# Langflow 成组故障恢复实施计划

> 使用 executing-plans 在用户已授权的当前工作区连续实施；保留历史修改，不提交、不上线。

**Goal:** 后端异常自动重启后，宿主恢复器发现实际网络命名空间失配，等待后端健康，按当前固定版本重建 LF，核验健康与真实新任务。

**Architecture:** 保留已验收的共享回环拓扑。Python 标准库宿主进程调用 Docker CLI，仅处理一个明确指定的 Compose project；不向 LF 挂载 Docker socket。只修复运行中的失配组合，不启动人工停止的服务。维护前必须停止恢复器。锁、冷却、超时与清单检查防止重复处理和错版本恢复。

**Spec:** 用户本轮请求、上一轮交付验收中记录的独立 app 重启故障。

## 约束与取舍

- 复用既有 Docker 自动重启 app；恢复器只重建 LF，不操作数据库、卷、其他项目，不下载/构建镜像。
- 以实际 `/proc/self/ns/net` 判断，不只看 container ID 或配置。修复成功还要求两个进程的 namespace 相同、LF health_check=200、app 代际未再次变化。
- 人工停机不被自动拉起；维护/版本切换前停止宿主恢复器，完成后重新启动。单实例 OS 文件锁避免两个恢复器竞争。
- 恢复不承诺续跑崩溃时已在执行的任务，不自动重放有副作用的工具。验收证明恢复后新任务成功。
- 接着完成本轮恢复器的打包与可安装服务说明；跨机镜像分发、完整前端/业务及备份恢复按前序交付顺序保留，不能假称已完成。
- 所有真实故障只注入本轮隔离 project；旧平台/CRM/MCP不参与注入。

## Tasks

- [x] 1. 编写恢复行为测试并确认RED：namespace失配、app未健康、人工停机、错项目/错镜像、版本变化、恢复后仍失配；实现宿主 recovery.py、带超时的Docker适配、单实例锁和退避，GREEN。
- [x] 2. 在隔离Docker栈复现后端异常重启；启用监控后再次注入，证明自动恢复、新任务成功、身份/图/数据保留，人工停机不会拉起。
- [x] 3. 将恢复器加入包白名单，补Linux systemd模板/维护说明，生成新不可变候选包并验证；一次fresh只读审查后处理重要问题。
- [x] 4. 停本轮监控/隔离服务，核验原服务、资源和卷保留；更新证据、接管手册和剩余交付项。

## Ledger

- Ruling：沿用共享namespace和宿主恢复器，而不是改远程能力桥或引入新编排服务；无需改变执行协议，代价是仍需短时恢复窗口与部署宿主Docker管理权限。该权限不交给LF。
- Ruling：用户已明确授权当前工作区实施及本轮故障验证，需求与故障具体，不重复询问实施许可；不提交、不部署生产。
- 核心测试先RED（无recovery模块）后GREEN。慢启动不循环重建与取消测试另先RED后GREEN；单实例锁在Windows与正式LF Linux测试镜像实际执行。最终恢复测试13项，交付测试新增1项，Linux全套39项PASS。
- 实机适配：LF没有Docker healthcheck字段，完整Go模板直接访问缺省map键会报错；已定位实际Docker stderr（无环境内容），改用index读取可选Health，实机通过。没有将基础设施/脚本适配失败归为产品失败。
- 两次实际SIGABRT注入仅对隔离project的PID1=WeKnora进行，不用docker stop/kill伪装异常；Docker自主重启同app ID，实际namespace失配复现。宿主恢复器两次重建LF，各次真实新任务1调用合成模型成功；LF人工stop持续8秒未被拉起。原型证据group-recovery-evidence.json；正式冻结包另复验。
- 一次fresh只读审查发现2项Important：env文件内容未锁定、可写payload启动产生pycache。两项均新增测试确认RED再GREEN；guard仅内存hash两份env并复核profile，入口禁字节码且systemd使用-B，不记录凭据摘要。无循环复审。
- 新包I/J包含恢复器、systemd模板和维护文档；旧包完整白名单仍可校验，恢复器两文件只可成组出现，部分混入拒绝。systemd仅模板，Windows宿主故障验收不等于Linux开机自启验证。
- 冻结I包入口再次完成两次SIGABRT恢复（不把原型验证代替最终代码），真实新任务各一次合成模型调用成功，人工停机保留，且安装目录未出现清单外pycache；--once=ready。故障后平台原登录、普通runner及私有图隔离仍有效，schema114/clean。
- I→J→I成组切换和回退均通过实际namespace、固定镜像、已有身份及真实新任务验证，14项Go运行时PASS。I SHA=5bff451c865f871ee04a1257835c4c7b7efec5e57ca3ba125bca79129d004a1b，J SHA=b504690da3b707fc07867513ccbe7f9782500295e7ece3774da0c36e7a4b8d6c；仍不冒称跨应用/LF依赖/schema兼容性。
- 收尾：本轮监控与隔离app/LF/PG/Redis已停，registry仍停，旧平台8080及原LF7861/health_check与前端5173均200；原CRM9999/MCP8765端口、Neo4j running保留。两只测试PG卷均保留。正式证据group-recovery-delivery-evidence.json及新验收文档。未提交、未外部发行、未上线。
