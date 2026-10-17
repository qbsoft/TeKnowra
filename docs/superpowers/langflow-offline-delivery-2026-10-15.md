# Langflow 跨机离线分发与安装回退验收

本轮完成集成运行栈的离线镜像运输、不可覆盖安装、成组切换/回退工具及本机独立接收端验收。未提交、未外部推送、未上线。不是完整 TeKnowra 企业业务或第二台物理机已验收的声明。

## 交付物

研究根：`D:/TeKnowra/research/selection/langflow-delivery-test-20261015`。

| 交付物 | 位置/标识 |
|---|---|
| 完整离线包 | `artifacts/offline-linux-20261015-b.zip` |
| 字节数 | 1,942,212,859（约1.81 GiB） |
| SHA256 | `4422cd3d4f2962b7a31530707b0e82b3ba6e384e2c35b56cbb92b4e472a2f8bd` |
| 配套受信安装工具 | `offline-installer-b/` |
| 摘要清单 | `offline-SHA256SUMS.txt` |
| 组件版本 | `linux-container-20261015-k`、`linux-container-20261015-l` |
| 最终机器可读证据 | `offline-delivery-evidence.json` |

包包含619个文件：四个固定Linux amd64镜像（app、LF、PostgreSQL、Redis）的可达manifest/config/layer，共80个blob；固定registry启动镜像的Docker-save归档；K/L组件ZIP；Linux依赖profile；宿主安装工具、只读registry配置及说明。app仍是之前实际构建的完整迁移版本，含Anydoc及BrowserSkill0.3.1。LF运行镜像config ID仍为52aa5b3e…，只是运输manifest固定为本地镜像引用。

**不携带**企业.env、模型密钥、OAuth、LF账号/图、数据库或业务状态。业务配置与状态由接收端提供。校验SHA检测传输变化，不代替可信发布渠道；没有宣称已实现签名发布体系。

## 接收端使用

要求宿主具备Docker Engine、支持`--wait`的Compose和Python3.12+，运行Linux amd64镜像。安装工具与ZIP一起通过可信渠道传递，先比对SHA，再执行：

```text
python -B <installer>/distribution.py verify --archive <offline.zip>
python -B <installer>/distribution.py install --archive <offline.zip> --root <distributions-root>
python -B <installed>/tools/delivery_manager.py import-images --installed <installed> --project <unique-import-project>
python -B <installed>/tools/delivery_manager.py switch --installed <installed> --root <component-root> --version <component-version> --locations <external-locations.json> --overlay <external-infrastructure.json> --project <runtime-project> --config <generated-compose.json>
```

占位值需替换，详细格式见包内OFFLINE.md与组件README。locations仅放外部路径；企业账号、服务身份、绑定和数据库初始化仍按既有管理流程配置。包不使用共用企业账号。

导入时临时registry仅绑定127.0.0.1:15000，挂载包内只读存储及固定配置。端口被占用就拒绝，不停止占用者。按不可变digest实际拉取四个镜像并核对config ID，随后移除自身临时容器。运行/恢复使用缓存与`--pull never`，不需要源仓库持续在线，也不在接收端构建镜像。

切换前先停宿主恢复监控；管理工具与监控共用OS排他锁，监控运行时拒绝切换。组件不可覆盖安装，activation在实际LF镜像中核对依赖。容器只写隔离验证目录，真实current.json由宿主原子更新；不会让root容器直接生成宿主普通用户不可读的生效指针，也不会在CLI超时后晚写覆盖真实状态。

切换保存旧指针和生成配置，等待基础设施健康，成组停止/重建app与LF，再核验实际namespace和健康。回退使用相同switch入口选择旧版本及其对应分发包。数据库不自动降级。

若启动失败，命令返回非零，恢复旧指针/配置，尽力停住失败的app/LF并保留transactions证据；**不把这个状态叫作业务已经回滚成功**。恢复外部配置后须显式switch旧版本，再验证真实任务。

## 实际隔离验证

接收目录为`offline-receiver/`，运行project为`teknowra-offline-receiver-20261015`，与构建/前轮project分离。源registry关闭；导入使用从包提取的独立只读registry，完成后删除临时registry。Docker层缓存仍是同一个本机daemon，但包内所有80个blob的内容摘要、引用图和启动归档均独立验证；没有把本机已有镜像名当作包完整性的证据。

接收端外部配置和状态复用此前独立的合成测试环境，用以验证原身份/数据保留；没有将这些配置和数据放进ZIP，也没有复制生产数据。PostgreSQL接收容器使用同一专属测试卷，旧测试PG保持停止，不会同时打开该数据目录。原本机开发数据库/CRM不参与故障。

已通过：

- 完整包校验、不可覆盖安装、四镜像按digest导入与config ID比对，临时registry清理。
- K→L→K组件升级/回退，每阶段健康、实际namespace、原登录、普通runner与私有图隔离检查；真实Go Engine→LF→合成宿主模型任务成功。
- 故意给接收端独有绑定配置置空，使新版本启动失败；返回非零、旧指针/生成配置逐字节恢复、app/LF停住、数据库不降级。恢复配置后显式切回K，新任务成功。
- 恢复监控运行时，switch被拒绝，原指针及两个容器ID不变。导入栈实际SIGABRT后，Docker重启app，监控重建LF，恢复后的新任务成功；此时临时registry已经关闭。
- 最终正式Linux LF镜像内53项Python测试、真实app容器内14项Go运行时测试通过。外部模型供应商、CRM和企业知识未被调用。

K/L只以组件注释标记区分，app/LF镜像相同。因此已验收的是组件包和部署状态回退机制，**不是不同业务二进制、LF依赖或schema版本的升级兼容性**。不同版本交付前仍须用对应候选另验。

## 审查与修正

唯一fresh只读审查发现5项Important：普通Linux用户指针权限、activation超时晚写、registry启动响应丢失清理、bootstrap缺层校验、内层tar解压界限。均有失败用例/最小复现后修正：宿主发布真实指针、隔离activation目录与唯一容器标记、按标记清理、完整layer/config/OCI校验及只接受有界未压缩Docker-save tar。Linux单测覆盖锁和归档分支；未将Windows测试等同于Linux开机自启验收。

实际首次导入又发现registry2.8.3不能以该嵌套环境变量方式设置readonly，报`readonly config key must contain additional keys`。已改成包内固定JSON配置只读挂载，启动200、四镜像拉取及修正版B整体导入均通过。失败候选A保留为诊断材料，禁止交付；清理A被自动审查拒绝（仅返回blocked by policy），未绕过或重试删除。

## 收尾与仍待验收

接收栈与前轮隔离栈均停止，监控、临时registry/activation容器退出，测试卷保留。原LF7861、平台8080与前端5173健康为200，CRM9999/MCP8765仍监听，Neo4j保持运行。C盘约9.3GiB、D盘约12.9GiB；后续大构建前必须重新核实空间，不能全局Docker清理。

仍未完成：第二台物理机/目标Linux宿主（含systemd启停与权限）的实装、对应正式前端和企业业务Linux浏览器验收、完整状态备份恢复、不同应用/依赖/schema兼容性及可复现构建流水线。本轮不触发生产安装或服务enable。
