# 可移交的停写备份与恢复

工具 `backup_restore.py` 仅需Python标准库和Docker CLI，没有D盘路径、开发账号或生产默认project。严格支持已验收的schema114、122，配置必须声明当前清洁版本；新版模板默认122，旧M检查点使用114。实际库、配置和备份版本不匹配时拒绝，不自动猜测或转换版本。它不自动停服务、不删数据、不覆盖目录、不自动降级、不重放中断任务；执行者先维护停写，并控制其他外部写入源。

## 备份范围

通过 `templates/backup.json` 明确project、写入服务、数据库和命名状态根。包括平台文件、发布库、bindings、普通服务身份、LF SQLite及加密密钥/配置。对象存储若是独立服务，由其维护方在同一停写窗口生成一致快照并记录恢复点；若是本机bind目录，把它加入state_roots并把minio加入writers。Redis不能作为唯一业务真相；正在运行的任务按中断处理，恢复新namespace，不承诺继续已有进程。

工具备份PG custom dump、停写状态ZIP、行数/schema、SHA及私有技能镜像清单。备份**包含密钥和业务数据**，目标目录自动收紧权限；不进入通用源码/运行交付包，不上传公共仓库，跨机器传递使用受控加密介质。校验和不是签名，移交时另存可信manifest摘要。不要在共享终端输出env、API key或完整dump。

## 备份操作

1. 填真实配置；用明确project与所有Compose文件定位部署，不使用模糊容器名。停宿主恢复监控，停入口、app、LF及所有写入相关存储目录的服务；PG保持运行供逻辑备份。禁止同时进行其他管理员操作/数据库写入。
2. 核对至少app/LF已停止，若本地对象存储入备份则minio也停止。配置的writers每项都必须存在且停止，否则工具拒绝。PG必须是同project唯一postgres服务。
3. `python3 -B backup_restore.py backup --config /etc/teknowra/backup.json --out /secure/backups/<unique-name>`
4. `python3 -B backup_restore.py verify --backup /secure/backups/<unique-name>`
5. **技能镜像另行导出**：读取受限backup.json里的skill_images，按记录的config ID，用 `docker image save -o /secure/backups/<unique-name>/skill-images.tar <id1> <id2>` 保存所有快照；为tar单独记录SHA256和大小。新机器load后逐一核对实际ID，并保留平台配置引用所需标签；不得只把ID写进文档就称镜像已转移。空闲沙箱容器无需搬迁。
6. 保存外置对象存储恢复点、对应M运行包清单、前端版本和平台/MCP回调配置说明。独立验证完成后按原配置启业务并进行一次新任务，再启动监控。工具不会擅自恢复入口。

恢复包必须与上述技能镜像归档、外部存储快照一起移交。`verify`只验证平台dump/state ZIP，**不表示外部存储或技能镜像已准备好**；backup.json明确skill_images_exported=false，操作人员另有验收记录。

## 恢复操作（隔离目标优先）

1. 核对备份可信来源/摘要，先verify。准备不同project、**全新PG卷**、全新数据库名和不存在的新状态目录；原目录/原数据库/失败恢复目标全部保留。固定镜像/profile使用M集合，先导入技能镜像并恢复对象存储。
2. `python3 -B backup_restore.py restore-state --backup <backup> --target /srv/teknowra-restored`
   写入前校验ZIP所有路径/成员/摘要，拒绝重复成员、路径穿越和链接。目标已存在则拒绝，不自动合并。Linux状态根0700、文件0600；之后按目标容器UID/GID配置必要权限。
3. 根据新目录建立目标独立Compose及backup配置：writers指向目标已创建且停止的app/LF（可用受控Compose create），postgres指向目标运行中的固定PG镜像。不要拿源project配置执行恢复。
4. `python3 -B backup_restore.py restore-db --backup <backup> --config /etc/teknowra-restored/backup.json --database <new_db_name>`
   目标库必须不存在，用template0创建，再pg_restore，最后核对备份声明的schema（114或122）clean和12张核心表行数；dump包含完整数据库。失败不删除新库，修正后使用另一个新库名，禁止drop源库后重试。122升级回滚到旧M时恢复升级前114检查点到另一个全新库/卷，再选择M程序与旧配套状态；不能把旧M接到122，也不运行down迁移。122运行后新增数据继续保留在122库，不会自动反向合并。
5. 将目标app DB_NAME、外置路径、Redis namespace、存储endpoint、域名/OAuth允许回调改为目标环境；**保留原AES/JWT/LF secret和服务账号API key**。Compose/env_file变化必须重建容器，不只restart。
6. 先检查实际镜像、挂载、UID/GID、PG/存储、LF身份及代理，再启动app/LF/frontend；旧宿主sandbox ID跨机不存在时应新建，不依赖ID不变。用低权限真实账号验证原会话续轮与首次技能脚本实际exit0，再验CRM按人授权。新域名/密钥更改引发重新OAuth时走本人授权，不能借用管理员token。
7. 验收通过后启动宿主恢复监控。失败时维持原环境可回退，不能用“current指针回退”代替完整业务恢复。

本工具不处理跨schema降级、分布式不停写备份、外部对象服务快照或secret管理系统。Linux目录所有权和所有外置资产齐备，是干净Linux环境的必验项。
