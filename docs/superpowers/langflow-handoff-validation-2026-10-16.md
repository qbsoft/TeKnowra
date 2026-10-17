# Langflow正式交付移交准备验收（2026-10-16，未上线）

**用户范围调整（2026-10-16）：** 用户明确取消干净Linux环境验收。该项不再作为本次交付阻塞，不继续分配VM或准备测试机；这属于取消验证，不代表该环境已通过。本文后续“下一关需要Linux环境”等保留为历史准备记录，以本段范围调整为准。M运行集合、N配套及其摘要不变；依旧不上线。

用户授权顺序：补齐部署依赖和配置、可移交备份恢复、对应源码归档，再准备干净Linux验收；保留全部未提交修改。已按此顺序完成本机可做部分，未修改应用执行逻辑或封存M集合，没有提交、外部推送、创建VM、启动WSL或上线。

## 新增配套交付

`D:/TeKnowra/research/selection/langflow-handoff-20261016/handoff-20261016-n.zip`

- 大小20,699,702字节。
- SHA256 `05e1120abb018a301deaf1444af11507467a42c64845187ebe8204795046d062`。
- 配套清单 `handoff-20261016-n/HANDOFF-MANIFEST.json`，SHA256 `3f701c025e4abd29203ad779b5b9cbdbec1e372e46ac4ecdc8833c82e451c2bf`。
- 24个文件的SHA256SUMS均核对通过。该配套**必须与原M集合同时移交**，不能把20.7MB配套ZIP当作运行镜像包。
- 原M集合17项摘要再次全部核验，未覆盖旧工件，也未改变原镜像身份。
- 通用配套不包含企业env、密钥、OAuth、数据库、对象存储、技能私有镜像或运行日志。

### 部署材料

仓库维护源：`integrations/langflow/delivery/`。

- `HANDOFF.md`：固定M单实例拓扑、责任/依赖清单、前端独立部署、平台/LF身份初始化、目录和UID权限、Docker mTLS沙箱接入及systemd维护顺序。
- `templates/`：六路径locations、受限infrastructure overlay、前端overlay、四类env文本、服务身份和绑定参考、备份配置、dependency-inventory，共12个无凭据模板；另带原systemd模板。
- `BACKUP-RESTORE.md`：停写窗口、备份范围、私有技能镜像导出、外部对象服务恢复点、原密钥保留、新卷/新目录/新库恢复、人工校验与启动顺序。
- `LINUX-ACCEPTANCE.md`：空缓存导入、空白身份初始化与恢复路线区分、真实业务、首次沙箱执行、CRM目标回调、宿主重启、异常恢复、回滚、新M备份恢复的验收表。

独立审查确认Redis启动命令的宿主Compose插值问题。先以真实Compose config、无宿主REDIS_PASSWORD而service.env_file带合成密码的测试复现失败，再将命令改为 `$${REDIS_PASSWORD}`，全套回归通过。未把该模板缺陷带入封存配套。

### 可移交备份恢复工具

新增 `delivery/backup_restore.py`，只依赖Python标准库和Docker CLI。backup/verify/restore-state/restore-db四个子命令，路径、project、服务和数据库由外置JSON指定，不写死D盘/开发账号。

- 通过Compose project/service标签核对唯一PG容器和已停止writers；不自动停/启业务，不误操作其他项目。
- ZIP全量摘要和路径校验在目标创建前完成，拒绝路径穿越、重复成员、链接、文件目录冲突及已存在目标。
- 恢复库必须不存在，template0建库；schema114 clean和12表行数核对，失败不删除源或失败目标。
- 备份/恢复目录收紧ACL，SQLite校验连接显式close；公开错误不包含Docker敏感stderr。
- 私有技能镜像清单属于备份，镜像另外导出及核验；verify平台ZIP不等于外部对象或技能资产也已验证。文档明确此界限。

首轮8项测试在工具缺失时失败，实施后通过。新增Compose回归1项，Windows完整Python套件62项通过；Linux容器内8项备份工具测试通过。Linux容器测试不代替Linux宿主验收。

真实演练从已停止的M恢复栈 `teknowra-release-restored-20261016` 仅启动PG，其六项writers保持停止。用新CLI生成新受限备份，再恢复到全新目录及独立project `teknowra-handoff-restore-20261016` / 新卷 `..._fresh-pg` / 新库 `handoff_restored`。目标app/LF只创建占位容器供作用域验证，从未启动。

114个状态文件逐摘要恢复通过，SQLite integrity_check=ok、旧三条trace保留，schema114 clean、12表行数一致。另实际导出172,004,173字节的私有技能镜像归档并load，1个快照实际config ID核对通过；同daemon缓存，不冒充跨机导入。备份和恢复数据在本机受限目录，与通用配套分离。本轮没有重复跑浏览器业务链；首次技能的真实业务依据仍为已通过的M轮。

### 对应源码归档

- `source-m-20261016.zip`：20,883,041字节，SHA256 `5bc9a60404087ac93d698e803eac82b427f5801b67bf9475fcaf124a01280941`，4,433个有摘要的文件。
- 后端从实际不可变编译阶段 `cad3106f583b…` 导出，而非只取当前HEAD，叠加冻结沙箱修复覆盖层；与旧构建清单比较，无缺失Go/SQL/Rust等指定编译源。
- 885项前端源文件与原构建指纹全部匹配；额外构建辅助文件明确标注current而不冒充历史指纹。精确前端运行context/dist和已安装M组件payload也已归档。
- `source-build-support.zip` 补充实际公开CA与完整runtime migrations构建输入，逐文件摘要；不包含私钥。
- `SOURCE-ARCHIVE.md` 说明如何组装源树、维护新版本，以及原本机构建标签、工具链/供应商缓存、字节可复现性等边界。完整第一方源与依赖锁已保存，但未验证重编译，也不承诺离线源码构建或字节级复现。

## 干净Linux准备结果与剩余阻塞

已经形成配置/依赖模板、源码和运维工具、固定版本及逐项验收步骤。干净Linux**实际验收尚未开始**。资源探测曾见本机仅0.58GiB空闲物理内存、C约2.92GiB、D约9.86GiB，不能合理追加建议8GiB RAM/40GiB可用盘的测试VM。WSL仅列举：Ubuntu停止、docker-desktop运行；未启动现有Ubuntu，更不能假定它是干净宿主。

下一关需要独立非生产Linux VM/测试机或重新明确的本机资源预算。必须验证目标Linux权限/mTLS沙箱、初始普通服务身份、systemd和宿主重启、空缓存安装及真实前端链路。上述是环境/验收阻塞，不能以本轮文档、Docker Desktop测试或旧账号恢复代替通过。客户域名/回调和资产inventory到环境选定后填写，不在通用包放客户秘密。

## 收尾

两个本轮使用的PG测试project已停止，所有原卷、新卷、私有备份和失败记录保留。原Langflow7861、app8080/health、前端5173和MCP8765元数据均200；未改MCP配置，未替换原开发app。本轮通用配套大约20.7MB、私有技能归档约172MB，均在D；不做全局Docker清理，不操作其他项目。

证据：`research/selection/langflow-handoff-20261016/` 内 `portable-backup-evidence.json`、`source-archive-evidence.json`、`linux-preparation-evidence.json`、`sealed-handoff-evidence.json`、`final-environment-evidence.json` 和 `logs/python-final.log`。
