# 双引擎生产上线与旧 CPU 兼容运行镜像计划

> 执行方式：superpowers:executing-plans；用户授权在当前工作区直接实施，保留已有修改。

目标：保持 Langflow 1.12.5，兼容生产 Xeon E5-2670 v2，完成 Eino 与 Langflow 生产配置和公网只读验收。

事实：官方固定镜像使用 UBI10，连 /bin/sh 都因 x86-64-v3 启动失败；平台48c31b5、MCP45c0bc6已在生产运行，schema122/fork2 clean。

约束：不在生产构建镜像；不复制本机测试库、OAuth、账号和流程绑定；保留原业务数据；无交付包；Langflow 不公开裸编辑端口，普通 editor/runner 分离，禁止自动登录/注册；共享 namespace 与 app 更新成组恢复。

- [ ] 在专用构建分支准备 Debian/Python3.14 兼容镜像，固定 Langflow/lfx 1.12.5，并以既有 Linux依赖清单约束依赖；CI执行现有模型/知识库/MCP/技能/多专家守卫测试，导出真实组件profile；镜像用不可变标签和digest。
- [ ] 在 CI 验证启动，再在生产空网络临时容器运行 Python/profile/组件导入，确认旧CPU兼容，无业务数据挂载。
- [ ] 服务器受限目录生成新装私有身份和持久存储；普通 editor/runner 的权限分离。复用现有 compose，以最小覆盖增加 Langflow 和 app 必需的只读资源及发布状态，先验证有效配置不改其他项目。
- [ ] 备份现行运行配置、数据库与回退镜像；成组更新 app/Langflow；回环编辑端口仅经 SSH 研发访问；自动发布后复核共享namespace关联并恢复Langflow，旧常驻watchtower保持停止。
- [ ] 公网 admin 从页面创建专用只读 Langflow 与 Eino 验收 Agent，复用生产现有模型、知识库、本人 MCP，不改旧Agent。验证保存/流程发布/执行记录和权限；失败不放宽规则。必要登录由用户完成。
- [ ] 核对运行镜像、前后端提交与schema，记录确实完成的范围、剩余限制及回退步骤。
