# Langflow技能接入实施计划

> REQUIRED SUB-SKILL: superpowers:executing-plans；用户已选择当前工作区实施，保留未提交修改，不提交、不生产部署。

**Goal:** Langflow可视图选择TeKnowra已安装/当前可用技能，按需读正文和资源，按授权执行技能脚本；复用平台技能管理、Agent选择范围和执行环境，真实场景验收。

**Architecture:** Binding增加skills及sandbox_config_id（脚本另有显式allow_skill_scripts上限）。目录返回已绑定且Agent允许的技能元数据，绝不返回环境变量/凭据。TeKnowraSkills组件输出按需读取和受控脚本工具；每轮宿主复用SkillManager/ReadFileTool/ShellExecTool，调用前后重读当前身份、Agent技能选择、已安装技能的ServedView和有效执行镜像。脚本必须属于该技能、限Python/Node/Bash，宿主构造引用安全命令，不接收自由shell。技能包/正文摘要纳入发布依赖，依赖变化拒绝执行/激活；旧无技能绑定行为保留。

**Boundaries:** 不以技能授予模型/知识/MCP权限，不让Langflow执行本机Python；复用按人的环境注入，不复制环境变量到图或目录。仍本机回环、文本、当前工作空间；不扩共享/长期记忆/任意shell。技能与脚本作者及现有执行环境属于现有平台信任边界，不宣传技能文件系统隔离。脚本产物沿用平台已有收集路径；stdout业务结果先验收。

- [x] 1. 定义/目录/发布校验：skills、sandbox身份、技能节点与摘要；兼容无技能绑定；缺绑定、重复名称、错误节点/动态凭据拒绝，先RED后GREEN。
- [x] 2. 宿主服务：复用有效已安装技能读取与原生管理器/受控执行工具；当前Agent选择/工作空间/包版本每调用复核；允许技能撤销/更新终止；脚本只接受技能内相对入口和参数。
- [x] 3. 执行桥与Python组件：/skill限定节点和绑定名称；schema/大小/调用预算，真实技能事件与版本；可视目录刷新、工具别名提示与requires_tools可用性提示；错误不静默降级。
- [x] 4. 本机管理/UI：复用已有自检技能验证L2/L3；在平台登记安装tyer-*只读业务验收技能，Agent界面选择；新技能版本发布，实际知识检索与脚本输出。验证未绑定技能、路径越界、技能禁用拒绝，恢复测试权限，保留原流程/Eino。
- [x] 5. 一次fresh只读审查、一次修复回归，浏览器+DB+trace证据及限制说明。

**Files:** langflowdef/hostcapability/langflowcatalog、langflowrelease、langflowruntime skill handler；application/service/langflow_skills.go及目录/发布/权限适配；integrations/langflow/components/teknowra/skills.py；repo/skills/preloaded/tyer-*验收包。适配代码不重写平台技能安装器或编排器。
