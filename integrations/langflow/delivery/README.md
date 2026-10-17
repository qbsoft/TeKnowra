# Langflow 成组交付工具

本工具安装的是 TeKnowra/Langflow 集成版本，不替代平台已有数据库、技能镜像和前端发布流程。单实例正式拓扑是平台与Langflow容器共享网络命名空间；短时任务能力仍只监听127.0.0.1，不发布随机回调端口。编辑入口仅绑定宿主回环，远程访问由现有受控网关或运维隧道承接。Langflow不挂载平台环境、OAuth存储或Docker socket。

## 包与安装

使用经过信任的构建渠道取得ZIP。SHA只能检测变化，不能证明发布者身份。release.py使用固定白名单，排除.env、图、运行数据、凭据和日志；组件、支持模块、启动器及依赖指纹随版本冻结。native-validation包不能称为正式Linux容器包。

命令均通过已安装且匹配的Python执行（占位路径需替换）：

```text
python release.py capture-runtime --library <site-packages> --output <runtime-lock.json>
python release.py build --source <integrations/langflow> --lock <runtime-lock.json> --version <release-id> --output <bundle.zip>
python release.py verify --bundle <bundle.zip>
python release.py install --bundle <bundle.zip> --root <new-install-root>
python release.py activate --root <new-install-root> --version <release-id> --library <site-packages>
```

每个版本安装在releases/<ID>，不覆盖同名内容；current.json原子切换。回滚使用activate选择旧版本，并先恢复相匹配的Python运行环境。切换指针不负责热替换已运行进程，必须停止本安装实例、核验后启动；禁止用PID模糊匹配停止其他服务。图版本回退仍使用平台现有发布管理；运行依赖版本回退需一起恢复，不能只回退图。

平台工件参数 --platform-artifact 可将已经构建的Windows后端随native验证包锁定，不将Git HEAD冒充实际构建产物。容器交付必须具备本轮平台镜像与LF镜像的不可变digest，以及从该LF镜像导出的关键源码/元数据profile；没有这三个工件不允许执行容器交付。

## 原生隔离启动

```text
python <installed-release>/payload/delivery/runtime_launcher.py --root <install-root> --library <site-packages> --state <new-state-root> --identity-file <external-restricted-identity.json> --port 17861
```

身份文件仅在版本目录外保存bootstrap用户名/口令与secret_key；secret_key必须是32字节随机数的URL-safe Base64编码，由部署方限制为服务用户可读。启动器核对依赖目录属于当前Python解释器、实际Langflow导入位置，拒绝将状态写入安装根的任何releases子目录。不继承平台数据库密码、模型密钥、原LANGFLOW配置；关闭auto_login与注册，使用独立SQLite与组件目录。bootstrap只用于初始化/维护管理员；编辑与运行身份必须分别创建为普通账号。禁止将身份文件复制到版本包。

## 容器模板的先决条件

compose.yaml使用JSON语法（Compose可读取的YAML子集），是独立overlay模板，不直接作为受控部署入口。容器包build时必须同时传 --app-image 和 --langflow-image，二者均为不可变digest；必须在目标Linux运行环境捕获依赖锁。安装/激活后使用：

```text
python release.py container-config --root <install-root> --profile <matching-image-profile> --locations <external-paths.json> --output <generated-compose.json>
```

该入口只读取并核验current.json指向的完整包，从manifest固定两个镜像和组件目录，核对profile版本/关键源码指纹；不能用环境变量覆盖版本或混入另一个组件目录。native-validation包拒绝生成容器配置。locations文件只能包含APP_ENV_FILE、LANGFLOW_ENV_FILE、BINDINGS_DIR、SERVICE_CREDENTIALS_DIR、APP_RELEASE_STATE_DIR、LANGFLOW_STATE_DIR这六个外置路径；其中平台环境与LF专用bootstrap配置必须分开，不能放在任何已安装版本内。调用只生成配置，不启动服务。

必须指定独立project名。profile应从与清单相同的LF镜像构建环境提取，包含三个lfx关键源文件及依赖METADATA，版本集合和源码SHA必须匹配runtime-lock；不能人工凑版本，也不把指纹校验称为远程镜像证明。单独替换app容器会改变共享namespace，升级/回退必须成组重建app和langflow。正式使用前以同一平台/镜像完成全栈验收，不能把Compose静态校验或网络探针当作全栈成功。

发布库保持单实例文件；数据目录与不可变版本目录分离。升级前停对应服务并备份状态，尤其SQLite、发布库、Flow和外置服务身份。跨数据库schema降级不自动执行；失败时按备份恢复相容状态，不覆盖现有业务数据。完整备份恢复、资源压测和重启任务收敛另有验收要求，不能凭安装包测试声称完成。

## 后端异常重启后的宿主恢复器

Docker独立重启app可能改变其实际网络命名空间，而已运行的LF仍留在旧命名空间。不能只检查容器ID或network_mode配置。`recovery.py`运行在部署宿主，使用Python标准库和Docker CLI，检查明确指定的单实例project、固定镜像、后端健康和实际`/proc/self/ns/net`；失配时仅按原清单重建LF，随后验证健康与namespace。它不重建数据库、不删卷、不拉取或构建镜像，也不把Docker socket交给LF。

```text
python -B <installed-release>/payload/delivery/recovery.py --root <install-root> --profile <matching-linux-profile> --locations <external-paths.json> --compose <generated-compose.json> --overlay <infrastructure.json> --project <explicit-project-name>
```

参数均为宿主路径；overlay是JSON格式的受信基础设施配置，只可为app添加数据挂载和依赖，不能替换app/LF镜像、LF配置或组件挂载。先按安装启动顺序使两服务运行，再启动监控；`--once`可执行一轮检查/恢复。监控使用安装根`.recovery.lock`的OS排他锁，第二个实例拒绝运行；部署账号须有该文件的写权限。版本目录应只读，恢复器同时禁止生成字节码，避免污染严格文件清单。

配置、活动版本、两个外置env文件内容或runtime profile改变后，监控拒绝继续恢复，须完成受控维护后重新启动；环境文件仅在内存计算摘要，不打印或持久保存内容/摘要。Docker失败输出不写入恢复事件，事件只含状态码和project。恢复超时带30至300秒退避，避免循环重建。Langflow正常慢启动只等待；人工停止或暂停任一服务不会被拉起。

**维护约定：先停宿主恢复器，再操作Compose或切换版本；完成成组启动、健康及一次新任务验证后，再启恢复器。** 人工维护与Docker重建没有跨进程原子事务，不承诺在任意并发运维操作下判断人工意图。恢复器不重放崩溃时正在执行的任务；已有任务须按失败/中断处理，业务侧不能据此自动重试有副作用工具。

`teknowra-langflow-recovery.service`是Linux systemd模板。安装前替换RELEASE_ID和路径，单独配置宿主服务账号、env只读权限与锁文件权限；账号的Docker管理权限等价于高权限宿主管理能力，不能与企业LF用户或模型工具共用。模板不会自动创建账号、安装Docker或启动业务栈。部署方核验模板后才能安装/enable；本机验收不安装生产服务，也不声称Windows故障注入已验证Linux开机自启。
