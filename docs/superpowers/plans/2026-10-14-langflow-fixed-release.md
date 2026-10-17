# Langflow 固定版本与服务认证实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将两个已验收的Langflow场景绑定到固定运行工件，补齐发布、激活、回退与服务认证。

**Architecture:** 私有编辑Flow与私有运行Flow由不同普通Langflow身份管理；宿主发布库保存不可变绑定与图/依赖指纹，活跃指针原子切换。每个Engine持有版本副本，认证调用官方API，组件仍只取得原员工任务能力。

**Tech Stack:** 既有Go/Gin/Vue、Langflow1.12.5官方API、Python/httpx现有环境；不装新框架。

**Spec:** 2026-10-14-langflow-release-design.md。

## Global Constraints

- 中文；事实优先；当前checkout直接实施，不提交、不推送、不生产部署，不覆盖历史未提交修改。
- 保留Eino和旧原型；不扩展多轮、技能/共享、SSO或跨容器部署。
- 凭据不进入图、发布清单、日志或说明；本机服务凭据保存在已有测试环境data下受限本地文件，仅服务器加载。
- 普通编辑者与运行账号分开，关闭自动登录/公开注册；管理员与OS管理员为可信运维。
- 构建和测试目录在D，限制内存与并行，不全局清理Docker，不删持久数据。
- 同次任务版本固定，资源授权始终重新检查；信用客户ID为字符串，MCP仍本人OAuth。

## Review Focus

- 生成运行Flow失败、发布库写入失败不能改变活跃版本：任务1/3。
- 发布后草稿修改、任务期间切换、回退后的新任务分别使用正确版本：任务1/2/5。
- 目录继续指向草稿，运行绑定指向发布版本，两个用途不可串用：任务3/4。
- API key或跨Agent/空间输入不能发布、切换别人的工件：任务3/4。
- 组件源码/工具schema/运行图变更不能静默沿用旧发布：任务2/3。

### 任务1：发布模型与持久库

**Files:** 新增internal/agent/langflowrelease/store.go及测试；修改langflowdef/binding.go、config/langflow_runtime.go及测试。

**Interfaces:** Release{ID,AgentID,WorkspaceID,EditorFlowID,Binding,GraphDigest,ToolsDigest,ComponentsDigest,RuntimeOwnerID,CreatedAt}；Store.Open(path)、Add(release)、Activate(agent,id)、Active(agent)、List(agent)，全部返回副本。Config分开DraftLangflowBinding与运行LangflowBinding，运行绑定活跃版本动态读取。

- [ ] 写并看到测试RED：原子切换、不可覆盖版本、异Agent拒绝、持久恢复、取得副本后切换仍V1、损坏库拒绝。
- [ ] 实现校验/有界文件/互斥/临时文件原子替换；写入失败不改变内存。开启发布库模式后未发布Agent不降级运行草稿。
- [ ] 测试GREEN，旧绑定/config/EinoDefinition回归通过。

### 任务2：认证客户端与版本固定运行

**Files:** 新增internal/agent/langflowapi/client.go及测试；修改hostcapability/options.go、langflowruntime/engine.go及测试。

**Interfaces:** Client.GetFlow/CreateFlow/Run；仅固定服务器地址、禁重定向，错误不携带凭据或原始响应。Options提供服务key及发布指纹检查回调，绑定带ReleaseID/GraphDigest；本阶段loopback拓扑保留。

- [ ] 测试RED：服务认证无auto_login、错误key拒绝、运行图摘要不符拒绝、发布绑定固定且模型/工具能力仍受限。
- [ ] 使用X-API-Key调用官方接口；发布前后规范JSON摘要算法共用，运行前检查所有者/图/schema/组件指纹；开始/完成事件记录版本，输出错误不泄密。
- [ ] 既有协议/取消/Eino测试及新客户端测试GREEN。

### 任务3：宿主发布与回退服务

**Files:** 新增service/langflow_release.go、langflow_release_validation.go及测试；修改langflow_catalog.go、agent_service_langflow.go、container.go。

**Interfaces:** Publish(ctx,agent,label)、List(ctx,agent)、Activate(ctx,agent,releaseID)，复用目录服务的人类编辑授权；读取固定草稿、读取当前已准许工具schema，导入新运行Flow后读回记录；激活前仍检查权限和工件。

- [ ] 测试RED：外部子流程/动态变量/未知组件/临时能力拒绝；绑定资源与实际图匹配；失败不切换、跨Agent拒绝。
- [ ] 发布只读取服务器声明草稿；锁定三类组件源码与支持模块、本机安装版本、工具schema；导入私有运行Flow与元数据原子保存。目录使用草稿绑定，运行使用活跃发布副本。
- [ ] service及Eino/Scope针对性回归GREEN。

### 任务4：产品研发入口

**Files:** 修改handler/langflow.go、router/routes_langflow.go、frontend/api/langflow.ts、LangflowIntegrationPanel.vue；新增必要测试。

- [ ] 测试RED：发布/切换路由API key默认拒绝，异空间不可访问；前端请求只传标签/已有版本ID。
- [ ] 增加当前发布版本、发布当前草稿、版本列表与激活/回退；已发布后打开仍是编辑草稿；试运行显示实际发布版本。
- [ ] 前端针对性测试、路由检查GREEN；真实界面验收。

### 任务5：本机身份隔离与全轮验收

**Files:** integrations/langflow本机认证启动/初始化辅助、README；dev-app.ps1本机凭据参数；研究目录新增隔离验收脚本/记录，不保存凭据进Git。

- [ ] 先测试配置校验：服务认证开启必须有key与发布库，旧显式原型仍单独保留但新发布模式禁止测试自动登录。
- [ ] 通过现有管理会话初始化普通编辑/服务账号、分别API凭据，复制独立编辑图，保留原图；关闭auto_login/注册，刷新浏览器为普通编辑身份；平台凭据仅本机受限文件。
- [ ] 实际匿名/错误key拒绝、普通编辑者修改运行Flow拒绝；V1→改草稿→仍V1→V2→回退V1；确定性测试模拟已创建任务保持V1。
- [ ] 两个现有场景正式聊天，查实际版本、工具、引用与保存；不再次扩业务场景。更新证据/交付说明。
- [ ] 一次末尾独立只读全变更审查，重要项一轮RED→GREEN修复；完整针对性回归后交付，无提交/上线。

## 执行

沿用用户此前明确选择的当前工作区直接实施；“开始”为本阶段授权，连续完成以上任务，不重复询问执行方式。计划自查依赖：Store/Binding供运行和发布消费，客户端供宿主服务复用；先分清草稿与运行读取，再接UI。新账号仅持有本机Flow管理能力，不授予企业业务数据权限；管理员绕过及分布式网络不作本阶段通过结论。
