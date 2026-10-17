# SDD ledger — plan: D:/TeKnowra/repo/docs/superpowers/plans/2026-10-14-langflow-multi-turn.md

- Scope：用户“继续下一轮”延续已授权当前工作区实施方式，先文本多轮。不提交、不生产部署、不改现有Eino或原流程。
- Ruling：复用宿主已有会话历史加载与多轮开关，原生Langflow历史关闭，避免切换Flow版本后上下文丢失及多轮关闭后仍有记忆。
- Ruling：历史仅最近20完整文本问答、64KiB；不回放工具、推理、多模态或压缩checkpoint。不宣传超长历史/长期记忆能力。
- Task 1 complete：过滤与完整问答限额/副本/指令顺序测试先undefined RED后GREEN。
- Task 2 complete：Execute带历史先“请使用新会话”RED；改为固定发布模式支持历史，每轮bridge独立；模型桥前置一次历史，全部原生Agent n_messages=0，ChatInput/Output不存消息。协议端点调用测试及整个langflowruntime包GREEN。
- Task 3 pending：已启动本机后端重建，正式聊天与会话隔离、开关、版本继续/多Agent验收待完成。
- Task 4 in progress：按executing-plans要求派唯一最末fresh reviewer，只读本轮范围；不派实现子任务。

- Native review：无Critical，三项Important已一次修复：发布/运行共用ComponentKind；专用LoadAgentTextHistory在附件展开前取原始用户文本与最终答案（普通/Eino历史路径不变）；按宿主TurnID合并全部用户指令并取末尾规范答案。对应节点后缀、追加指令、文本投影测试先RED后GREEN。
- Ruling：智能推理模式由EnsureDefaults强制多轮，无界面关闭开关；本轮不改变宿主既有模式语义。nil历史记忆关闭只由协议测试覆盖，不宣称界面关闭验收。
- Ruling：Langflow历史加载失败直接拒绝启动，避免数据库错误被静默当作无历史；其他引擎沿用既有fallback。文本投影不回放压缩checkpoint，当前范围是最近完整文本轮次。
- Task 3：初版实机同一会话新增项目代号及检索成功；已界面激活摘要V2，待最终修复版重建后续聊核验。该代号为用户会话描述，不应被包装成知识库事实。

- Task 3 complete：最终修复版真实同会话续聊成功，V1切V2记得临时代号并计算40000元；新会话不知道该代号；多Agent第二轮客户指代成功，重新四个工具span，两个Agent，CRM字符串ID保留。三次trace均ok，版本/Flow对应不可变工件。
- Task 4 complete：三项Important已一次修复并通过新旧针对性回归。没有工具的隔离回答agent_steps为空，版本证据单独标为trace_flow_id；不伪称步骤版本。
- Ruling：原生message计数续聊前后3→4，框架仍有执行消息/追踪落库。本轮保证不读取为历史，不宣传全部LF存储关闭；数据保留策略属正式部署。
- Final：摘要活跃V1已恢复，合同与信用原版本保留；没有新增发布或修改原流程。证据与限制见research/selection/langflow-decision-test-20261013/文本多轮验收记录.md。
