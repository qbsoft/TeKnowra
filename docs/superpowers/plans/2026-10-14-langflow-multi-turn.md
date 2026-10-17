# Langflow文本多轮实施计划

> REQUIRED SUB-SKILL: superpowers:executing-plans。沿用用户已授权当前工作区直接实施，保留所有未提交修改，不提交、不部署。

**Goal:** 固定发布的Langflow智能体可以在TeKnowra同一会话继续文本任务，历史来源唯一、会话隔离，关闭多轮时无历史。

**Architecture:** TeKnowra已有LoadAgentHistory从会话数据库加载已完成的历史。执行器按当轮保留最近完整文本问答，在模型桥中放到当前指令之后、当前任务之前；不重放旧工具调用、结果或推理。每个原生Agent的n_messages运行时固定0，ChatInput/Output不保存消息，避免Langflow另一套记忆。版本图仍不可修改，控制项只在当轮tweaks注入。

**Constraints:** 本机固定发布认证模式；无长期记忆、图片、多进程重启恢复或技能/共享。每轮重新授权资源，历史是用户可访问会话的文本记录，不能用作当前业务事实的授权证明。最多最近20个完整问答、64KiB；单个最新问答超限明确拒绝。切换发布版本后继续采用TeKnowra历史，新任务使用当前活跃版本。现有多轮开关决定宿主是否传history。

**Tasks**

- [x] 1. 新增history.go及测试，过滤系统/工具/推理/附件字段，完整问答限额，指令-历史-当前任务顺序与任务副本隔离；先RED后GREEN。
- [x] 2. Engine移除固定发布模式的history拒绝，将历史副本放当轮bridge；模型桥每次调用使用同一副本。认证callFlow从已校验图选择全部Agent/Input/Output注入关闭记忆项；协议测试验证实际请求与历史模型调用，旧无历史行为回归。
- [x] 3. 本机重建，在摘要Agent同会话两轮引用首轮临时项目代号/合同，新会话询问同代号不应知道；核实宿主模式开关：智能推理原本强制多轮，无界面关闭开关；仅验证nil历史协议，不新增开关。验证多Agent第二轮沿用客户指代、工具重新调用与发布ID保留；不改变原流程。
- [x] 4. 一次独立源码审查与针对性修复、回归，记录实际界面/数据库/trace证据和限制。

**Files:** internal/agent/langflowruntime/{history.go,history_test.go,engine.go,bridge.go,multi_turn_test.go}；integrations/langflow/README.md。不替换既有宿主历史加载、不改Langflow源码和平台默认Agent配置。
