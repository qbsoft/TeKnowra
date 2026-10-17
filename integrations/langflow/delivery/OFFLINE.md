# 离线交付包

本包面向Linux amd64镜像，接收宿主需要Docker Engine/Compose和Python 3.12或更新版本。不包含企业.env、账号、OAuth、数据库或业务状态；通过可信渠道校验包的SHA，SHA本身不认证发布者。

包携带固定镜像的manifest/blob、registry启动镜像、组件版本、Linux依赖profile及宿主管理工具。导入期间使用127.0.0.1:15000临时只读registry；端口必须空闲，不停止其他占用者。导入后关闭registry，运行使用已导入的固定digest。

安装目录不可覆盖。运行配置、服务凭据、发布库和数据库状态放在包外。维护前先停宿主恢复器；切换工具使用同一恢复锁，监控仍运行时拒绝切换。业务数据库不自动降级。

## 校验、安装和镜像导入

交付方同时提供受信的installer目录（四个Python工具）和整个离线ZIP的SHA256。先通过可信渠道比对摘要，再执行以下命令；工具本身也属于可信交付链的一部分，不能用未验证的未知脚本自证安全。所有路径是接收宿主路径，无需连接源机器。

```text
python -B <installer>/distribution.py verify --archive <offline.zip>
python -B <installer>/distribution.py install --archive <offline.zip> --root <distributions-root>
python -B <installed-distribution>/tools/delivery_manager.py import-images --installed <installed-distribution> --project <unique-import-project>
```

导入先核对启动镜像专用tag，拒绝覆盖同名异物；若缓存缺失则从包内Docker-save归档加载。临时仓库有唯一操作标记，只清理本次创建的容器；四个镜像全部按digest拉取并比对实际config ID后才成功。启动响应丢失也按唯一名称/标记核对清理。Docker不可用时不能保证清理完成，命令会明确失败，修复daemon后按操作标记处理遗留，禁止全局清理。

## 安装、升级和回退

接收端准备六路径locations JSON和基础设施overlay JSON，格式同组件交付README；overlay只能包含app数据挂载/依赖，以及postgres/redis，后两者必须使用distribution.json里的固定镜像引用。环境和服务账号配置由接收端持有。新装时LF服务账号/绑定的初始化遵循已有管理流程，不使用包内共用账号。

```text
python -B <installed-distribution>/tools/delivery_manager.py switch --installed <installed-distribution> --root <component-install-root> --version <component-version> --locations <external-locations.json> --overlay <external-infrastructure.json> --project <explicit-runtime-project> --config <generated-compose.json>
```

回退使用同一switch命令选择已安装的旧组件版本，并指定包含旧镜像/profile的分发包；先完成该包的镜像导入。它会获取与恢复监控相同的排他锁、核验镜像/profile，保存旧pointer/生成配置，等待基础设施，再成组重建app/LF并验证健康与实际namespace。不执行schema降级。

activation容器只在临时验证目录写指针，真实指针由宿主原子更新，避免root容器生成0600文件导致普通部署用户无法读取，也避免Docker CLI超时后容器晚写覆盖旧状态。临时容器按独立标记清理；验证目录和事务记录保留用于诊断。

若切换启动失败：命令返回非零，恢复旧pointer和生成配置，尽力停止本次app/LF并保留transactions记录；不自动拉起旧业务、不谎报回滚成功。恢复原配置后显式switch旧版本并执行一次真实任务验收。数据库和业务状态保持包外，不恢复或删除；不同schema/应用版本之间的兼容与备份恢复必须另验。

成功后按组件README启动恢复监控。Linux systemd模板的账号、路径与权限由接收方配置；本工具不自动enable任何生产服务。离线包不含正式前端，此阶段验收是集成运行栈，不能替代完整企业业务浏览器验收。
