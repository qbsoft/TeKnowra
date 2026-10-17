# 沙箱修复正式交付包验收（2026-10-16，未上线）

已将上轮恢复后首次技能执行修复固化为目录式交付集合 `teknowra-langflow-20261016-m`，完成新接收根安装、真实应用镜像回滚和新卷业务恢复后的首次技能任务验收。本轮没有进一步修改产品执行逻辑，没有提交、外部推送或上线，原工作区未提交修改全部保留。

## 交付工件与版本

集合目录：`D:/TeKnowra/research/selection/langflow-release-20261016/release-set/teknowra-langflow-20261016-m`。

| 工件 | 版本 / 摘要 |
|---|---|
| 主运行归档 | `offline-linux-20261016-m.zip`；2,034,426,246字节 |
| 主归档SHA256 | `ac3ffecba99e93e6c2545b110d5c16d2b3dc140e0099db603e719226203ef983` |
| 组件版本 | `linux-container-20261016-m` |
| app实际config ID | `sha256:69e17c85273afe75b89331e5c5862c9410c4cf94fa942ce5419274b131f9d23e` |
| app固定manifest digest | `sha256:e6723afdc88833a394601fd4155104ec75603ecb0ebffe7405b5e80bfd759b97` |
| 应用二进制SHA256 | `96ed83e3085092507c4a076b2540fd555c358573ccb3c21815b8ce0105812607` |
| 正式前端配套 | 原已验收 `frontend-linux-20261015-a.tar.gz`；config ID `sha256:e2db43ba58ba94f4e7f879118810a5b2aabf489f701c2fe33ef5cbfab7122167` |
| 回滚归档 | 原完整 `offline-linux-20261015-b.zip` 及原安装工具；SHA256 `4422cd3d4f2962b7a31530707b0e82b3ba6e384e2c35b56cbb92b4e472a2f8bd` |
| 统一版本清单 | `RELEASE-MANIFEST.json`；SHA256 `c529f4202ff1459ca40111a9bd9f066e30486611b0ac42d588e98bff6231be46` |

`SHA256SUMS.txt` 覆盖17个文件，包括版本清单。逻辑工件总计4,012,156,450字节（不含清单与SHA文本），主包约2.03GB。旧B归档在本机用同盘普通文件硬链接节省空间，字节未改；向其他磁盘交付完整目录时会复制完整回滚归档。

新镜像以原正式基底和上轮已验证候选中的应用二进制组成，实际比对二进制SHA相同，正式镜像不含 `/opt/teknowra-tests/sandbox-restore.test`。二进制内嵌版本仍为 `sandbox-restore-fix-20261016`、源状态 `dirty-workspace-sandbox-fix`；集合/OCI版本M不冒充重新编译版本或干净Git提交。两份产品源文件指纹已再次核对与上轮构建上下文一致。

四镜像安装器继续负责app/Langflow/PostgreSQL/Redis。前端为同一集合内独立配套归档，按README加载并接入业务网络，四镜像安装器不自动启动前端。企业配置、账号、模型密钥、OAuth、数据库、对象存储和技能快照全部外置，不进入通用集合。本机Docker relay仅用于本机验收，不随包作为正式组件自动安装。

## 隔离安装与回滚

在独立接收目录 `research/selection/langflow-release-20261016/receiver` 安装新ZIP；执行包内导入器核验四镜像，临时只读registry随后移除。独立project `teknowra-release-acceptance-20261016` 使用新数据库卷，未覆盖旧安装目录或旧归档。

| 步骤 | 实际app镜像 | 新Langflow任务 |
|---|---|---|
| 旧K安装 | `c226f1cdbe71…` | 成功 |
| 新M安装 | `69e17c85273a…` | 成功 |
| 回滚旧K | `c226f1cdbe71…` | 成功 |
| 再切新M | `69e17c85273a…` | 成功 |

每步核验实际config ID、app/Langflow共享网络命名空间、schema114 clean、原平台登录及普通runner身份。此阶段真实执行Langflow，但模型是明确的合成测试模型，每任务一次模型调用、不调用外部供应商。53项Python交付测试通过；首阶段14项Go运行集成测试通过，其余每阶段运行针对性Langflow任务测试。单次独立只读审查在指定范围内未发现可确认Critical/Important/Minor问题。

## 从业务备份恢复后的首次技能

将10月15日一致备份 `frontend-business/backup-01` 恢复到新目录 `restored-release-m`、新数据库卷和project `teknowra-release-restored-20261016`。116个状态文件逐摘要校验，12表恢复前行数匹配，schema114 clean。app实际使用新M镜像，另外七个服务镜像保持备份预期；实际挂载payload来自新接收端M组件，profile来自新分发包。前端配套归档已实际load并核对config ID。

正式前端中sales01继续原会话 `2c90ef32-f722-4eb0-8a34-fd725b16b381`。在执行前，核对仅该测试会话、tenant10001、受管沙箱 `e8816c7e4a4d` 的标签，将其activity marker设为三小时前，覆盖旧空闲TTL路径。未全局清理或删除沙箱。

- 首次任务即成功，无失败后重试；真实DeepSeek调用，读技能一次、执行脚本一次。
- `scripts/report.py` 实际stdout预付款金额 `60000.00`，exit0、stderr空、未killed，脚本约88ms。
- 原会话代号 `LF-DELIVERY-1015-X9` 保留。
- trace `7f522f6d50b64c7bac7454f5192c4c5a` 为ok，旧两条及新第三条trace全ok；一个真实script工具span。
- 仍用原沙箱，activity marker已经续活，覆盖本次修复的原始故障条件。
- 数据库只增加本次两条消息；其他11表行数不变，六条会话消息完整，schema114 clean。
- SQLite取证在Linux容器内只读连接并显式close，未在Windows宿主打开运行中SQLite。

界面证据：`research/selection/langflow-release-20261016/shipped-restored-first-skill.png`。

## 收尾与范围

两个本轮隔离project及此前三项业务/恢复project全部停止，所有卷、备份、失败记录保留。本机registry停止，包导入器临时registry已移除。原7861/health_check、8080/health、5173、8765 OAuth元数据均200，CRM9999监听；MCP配置未改，原8080未替换。最终C约4.62GiB、D约10.10GiB可用，没有清理其他项目。

本轮证明本机Windows Docker Linux amd64的安装/同schema114镜像回滚/既有业务备份恢复到修复版本/首次真实技能执行。使用同daemon镜像缓存，未证明第二物理机、空缓存初始化、不同schema迁移/降级、Linux宿主systemd开机自启；也未从M新建一份业务备份再恢复。SHA用于完整性核对，不替代发布者签名信任。没有上线。

全部机器证据位于 `research/selection/langflow-release-20261016/`：`runtime-image-evidence.json`、`lifecycle-evidence.json`、`release-restored-evidence.json`、`first-skill-evidence.json`、`sealed-release-evidence.json`、`final-environment-evidence.json`。含企业状态的受限测试目录与通用交付集合分离，不对外发布。
