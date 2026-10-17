# 跨机离线分发与安装回退计划

> 当前工作区 executing-plans 连续实施；保留已有修改，不提交、不外部发布、不上线。

**Goal:** 交付不依赖本机缓存/外部下载的镜像与集成版本包，在隔离接收目录和独立registry/project实际验证导入、安装、运行及回退。

**Architecture:** 固定四个Linux amd64镜像（app/LF/PG/Redis）镜像清单和层，以只读registry文件快照随包分发；另携带固定registry启动镜像Docker archive。接收端校验包后短时启动仅绑定127.0.0.1:15000的导入registry，按digest拉取并比对config ID，随后关闭registry。组件、Linux profile、宿主工具与镜像分离于外部凭据/业务状态。

**Spec:** 用户本轮要求与已验收的共享回环、宿主恢复器、不可覆盖组件包设计。

## 取舍与边界

- 不构建新业务镜像，不向外部registry推送。镜像源使用已核验缓存，只镜像至本机分发仓库。完整镜像引用统一为localhost:15000，避免机器地址硬编码；端口占用必须拒绝，不停止占用者。
- 离线包包括app/LF及隔离安装所需PG/Redis镜像，不含企业数据库、OAuth卷、.env、模型密钥、LF图或账号数据。初始化业务配置仍由接收端提供。
- SHA和镜像digest用于完整性，不代替可信渠道身份认证。依赖Docker Engine/Compose和Python标准库，镜像架构明确linux/amd64。
- 不可覆盖安装、严格文件集合、路径/symlink/大小/摘要校验；实际镜像manifest与可达层摘要核验，不因源daemon已有缓存就省略包内镜像数据。
- 管理命令获取同一恢复锁，监控运行时拒绝切换；切换前核验新包、profile与缓存镜像，备份旧指针/生成配置，成组停止/启动。启动失败保留事务证据并恢复旧指针/配置，停在明确失败状态，不自动降级数据库。
- 本机隔离接收端不等于第二台物理机；实际验证独立registry、不同安装根和不同Compose project。不把组件标记差异的回退冒称跨应用/schema兼容。
- C至少6GiB、D至少12GiB，重型分发/校验串行；不做全局Docker清理，不删旧数据库/OAuth卷。

## Tasks

- [x] 1. 单测RED→GREEN实现delivery/distribution.py：离线build/verify/install，固定工具/组件/profile/registry/bootstrap白名单，拒绝篡改、逃逸、额外成员、半安装和覆盖。
- [x] 2. 固定镜像镜像至本机registry，冻结新K/L组件包，导出bootstrap镜像及只读registry快照，生成完整离线包。
- [x] 3. 实现并测试import-images和switch；在新接收目录/registry/project实际导入、安装、运行、升级和回退，新任务与恢复器验证；故障注入验证失败状态。
- [x] 4. 一次fresh只读审查与重要项修补，最终回归、工件/资源/恢复记录。补交付使用说明，保留正式前端、企业业务、完整备份恢复的后续验收边界。

## Ledger

- Ruling：选只读离线registry快照而非仅docker save app/LF。后者跨机load未必保留原RepoDigest，会让固定digest清单失效；保存原registry manifest/blob并实际按digest拉取可保持内容寻址。代价是接收端导入期间需临时空闲15000端口。
- 完成：app/LF/PG/Redis仅镜像至本机仓库，config IDs与原始固定镜像一致，实际80个可达blob逐摘要收集；除固定bootstrap外，不把Docker缓存名称当可分发工件。原始构建/基础镜像未改变，未关闭Anydoc等功能。
- TDD：distribution从无模块RED到7项GREEN；manager从无功能RED到7项GREEN。activation响应丢失的初始用例曾被过早路径校验挡住，修正夹具后观察真实RED再修正，未把假阳性当覆盖。
- 一次fresh只读审查5 Important：普通宿主指针权限、activation晚写、临时registry响应丢失、bootstrap缺层、压缩tar资源界限。修正为隔离activation根+宿主原子发布、唯一操作标记清理、完整layer/OCI/config校验、未压缩有界tar。对应RED→GREEN，未循环复审。
- 实机问题：registry2.8.3 readonly嵌套env配置panic；诊断仅本轮无凭据registry，改固定JSON只读挂载后/v2=200且四镜像实际pull。首次失败A保留；清理本轮A被自动策略拒绝blocked by policy，未换工具绕过，改在D剩余预算内继续。修正版B包含619文件、1942212859字节、SHA4422cd3d4f2962b7a31530707b0e82b3ba6e384e2c35b56cbb92b4e472a2f8bd。
- 本机独立offline-receiver目录/只读registry/project验收完成：源registry停，四固定digest导入/config ID相符，临时registry清理；K→L→K各次真实新任务成功，原登录/runner/私有图隔离及schema114保持。接收端复用原有合成测试状态作为包外输入，不包含在分发包，也不是第二物理机或空业务状态初始化验收。
- 启动失败注入：接收端自己的binding置空，切换非零、旧pointer/config逐字恢复、app/LF停住、未降级DB；恢复binding后显式K恢复，新任务成功。监控持锁时switch拒绝且容器ID不变；导入栈SIGABRT后自动重建LF并成功执行新任务，此时registry已关闭。
- 最终正式Linux LF镜像内53项Python PASS，真实app内14项Go运行时PASS；同app/LF镜像的组件标记切换，不宣称不同业务二进制/schema兼容。新app/LF/PG/Redis及旧隔离栈均stop，临时容器清理，测试卷保留；原7861/8080/5173健康200，9999/8765仍监听，Neo4j running。C约9.32GiB、D约12.95GiB，不再新建大副本。
- 交付说明docs/superpowers/langflow-offline-delivery-2026-10-15.md，工件与详细证据在研究根offline-delivery-evidence.json、offline-SHA256SUMS.txt、offline-installer-b/。剩余：目标Linux宿主实装与systemd、对应前端/企业业务、备份恢复/跨版本兼容、可复现构建流水线。未提交、未外部发布、未上线。
