# 固定M版部署与移交基线

本配套不修改封存的 `teknowra-langflow-20261016-m` 集合。交付时同时提供M集合、handoff配套目录及可信来源的摘要。目标是Linux amd64、单实例、schema114；Docker Engine与Compose v2、宿主Python3.12+由接收方提供。应用镜像内部LF解释器版本和441个依赖由原profile锁定，宿主Python不替代它。

## 依赖职责

| 依赖 | 提供方及固定方式 | 必需条件 |
|---|---|---|
| app/Langflow/PostgreSQL/Redis | M主ZIP四镜像及安装器；按distribution固定digest/config ID导入 | 必需。PostgreSQL是清单内的扩展版，不能随意换普通postgres；不发布数据库/Redis宿主端口 |
| 正式前端 | M集合独立前端归档；config ID e2db43ba58ba… | 必需。与app同一业务网络，APP_HOST=app、APP_PORT=8080，重建app后重建前端或reload nginx |
| 对象/文件存储 | 客户既有S3兼容服务或独立受管MinIO，以及本地文件目录 | 按平台实际STORAGE_TYPE配置，备份桶/文件内容和访问配置；M不包含MinIO镜像，不把本机测试桶当正式依赖 |
| 技能沙箱基础镜像与各空间技能快照 | 按平台配置的实际镜像ID由客户/交付人员单独导出导入；属于私有业务资产 | 用脚本技能时必需；只导出基础镜像不够，backup.json列出空间快照；已有会话沙箱容器可丢弃重建，不能依赖其宿主ID跨机存在 |
| Docker沙箱访问 | 目标Linux部署方设置宿主Docker访问方案和app用户权限 | app固定非root；LF不挂Docker socket。四镜像安装器禁止通过app overlay挂/var/run；因此不能把docker.sock直接塞进该overlay。需预先提供受控Docker TCP端点并在平台沙箱配置中登记，限制网络可达范围及认证；不得裸露公网无认证2375。本机Unix relay是验收适配，不作为客户默认方案 |
| MCP/CRM与按人OAuth | 客户既有服务；配置真实HTTPS地址、client和允许回调 | 用CRM场景时必需；回调必须指向客户前端域名。恢复平台AES/JWT和OAuth加密所需原密钥，不能静默换共用账号 |
| 模型、embedding、rerank | 客户现有供应商/本地模型服务及授权配置 | 由平台统一管理，图不含供应商密钥；对应知识检索按平台模型配置检查 |
| 文档解析、图谱、搜索等 | 已启用功能的外置服务 | 仅启用相应功能时必需；本轮不新增功能，按实际客户场景填写inventory |
| 网关/TLS、备份介质、宿主恢复器 | 部署方 | 编辑入口17861/执行入口18080保持回环，仅受控网关/隧道访问；恢复器运行在宿主，不把socket交给LF |

`templates/dependency-inventory.json` 是交付前填写的责任清单，未填写项不能称为环境就绪；它不是新的一套编排平台。资源起点建议给独立验收VM 8GiB RAM、40GiB空闲磁盘；这只是测试环境预算，不是生产容量或并发保证。

## 路径与配置

- 不可变程序：`/opt/teknowra/distributions/offline-linux-20261016-m`、`/opt/teknowra/components/releases/linux-container-20261016-m`。
- 可变企业配置：`/etc/teknowra`，目录0700、文件0600；按容器实际UID/GID赋予最小读取权限。不能为了让容器读到文件直接chmod777。LF只读自己的env，不挂平台env、OAuth或Docker访问凭据。
- 可变企业数据：`/srv/teknowra`；各状态目录独立，发布状态和LF数据库不在程序版本目录中。
- 模板仅含替换占位符，`platform-env.txt` 不是完整平台所有功能的默认配置；以源码config与原平台配置为准。恢复部署必须保留原AES/JWT/LF secret_key，否则数据库恢复成功也可能无法解密。
- 使用 `locations.json` 六路径；`infrastructure.json` 只给app增加许可数据挂载、depends_on及固定PG/Redis。它不是全栈Compose，不能单独up。前端以独立overlay在同一project启动，不把它传入四镜像switch的overlay参数。
- Docker远程端点实际支持DockerTLSCertPath，目录必须含ca.pem/cert.pem/key.pem。若采用该方案，将该企业私有证书目录只读挂到app的 `/etc/teknowra/docker-tls`（合并进infrastructure的app.volumes，不能挂到受保护/run或/var/run），在平台空间沙箱配置登记容器内路径，并配置目标内网端点允许策略。LF不挂此目录；证书属于私有备份，不进本配套。网络/TLS/UID读权限需要在目标Linux实测。

## 安装与身份初始化

1. 在干净Linux上核对交付来源和M集合的SHA256SUMS，按M README完成verify、install、import-images。必须断开源构建仓库后仍能运行；导入所需15000回环端口应空闲。不要docker pull latest。
2. 按templates复制外置配置并填值，准备PG/Redis、存储、沙箱端点，核对相应UID/GID及磁盘权限。数据库全新安装与已有业务恢复是两条路线，不能混用测试账号/备份作为默认初始化。
3. 新装LF以外置bootstrap env创建初始管理员，关闭auto_login与注册。通过其管理界面建立**普通编辑账号、普通运行账号**，分别生成API key并填写service.json；runtime_owner_id填普通运行账号UUID。bootstrap仅维护使用，不作为TeKnowra执行身份。
4. 平台按原初始化流程建立系统管理员、工作空间、模型、知识库、MCP/技能及沙箱配置。研发以编辑账号导入/创建图；填 `bindings.json` 时使用本企业的真实Agent/Flow/资源/节点ID。空清单用于启动，不代表已配置Agent。不要复制旧客户的ID或目录连接票据。
5. 执行M README的switch；它核验镜像与profile并成组启动app/LF。首次设置时如尚未生成普通服务身份，可先用原受控启动流程初始化LF，写好凭据后再进入正式switch；不得为此启用匿名登录。已有业务恢复则直接还原原身份、密钥、bindings和发布状态，不重新生成。
6. 加载前端归档，并用 `docker compose -p <project> -f <generated> -f <infrastructure> -f <frontend-overlay> up -d --pull never frontend`。先核对实际app/LF/frontend镜像ID、namespace和API代理JSON健康，再经正式前端保存、发布、共享、真实任务验收。
7. 替换配套systemd模板中的版本/路径/服务账号；账号对安装根锁文件可写、对immutable版本只读，具备受控宿主Docker管理权限。**先业务就绪再enable恢复器**。维护时先停恢复器；镜像回退沿用M README并重建前端解析。权限边界不得因为编辑平台接入而扩大。

未执行过的初始化顺序、Linux权限与systemd宿主重启均列在 `LINUX-ACCEPTANCE.md`，不以此文档代替实测。
