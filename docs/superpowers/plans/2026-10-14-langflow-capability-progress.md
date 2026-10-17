# 实施记录 2026-10-14 Langflow 通用接入

计划：2026-10-14-langflow-capability-integration.md。用户明确选择在当前工作区直接实施，保留全部未提交修改。

- Pre-flight：任务1绑定类型由任务2/3/4共同消费；运行调用与目录票据分用途；Python DTO与Go目录形状须一致。
- Ruling：现有普通checkout位于cursor/mcp-oauth-return-to-current-page；按用户选择在此实施，不新建worktree，不提交历史修改。
- Ruling：仅实施A+B+C；固定版本、历史、技能/共享及部署仍留在后续，不默认扩范围。
- Task 1：complete；先出现ParseBindings/配置接口不存在的RED，实现后绑定/config/EinoDefinition测试GREEN。
- Task 2：实现和初测通过；不再依赖Eino定义，绑定按服务+工具定位；旧原型/Eino runtime测试GREEN，真实流程待验收。
- Task 3：实现和初测通过；知识能力从原权限实现抽出共用入口，MCP保留本人身份、参数校验和审批；Langflow/Eino/Scope针对性测试GREEN。
- Task 4：目录短期会话和独立DTO已实现；过期/撤权/未知票据测试GREEN，路由与API key相关测试GREEN；真实研发连接待验收。
- Task 5：动态组件已实现；原schema校验、稳定资源ID、服务工具命名、清空目录连接、嵌套参数与显式null测试先RED后GREEN；界面待验收。
- Task 6：创建独立知识摘要副本（8ba2de39-801d-4de5-a4ad-2c11d1730fe5），旧Eino Agent保留；准备本机重启和双流程验收。
- Ruling：目录连接输入刷新后清空，真实票据只留Langflow进程内、按Langflow用户和TeKnowra Agent隔离；仍需读回保存图确认无票据。重启或15分钟到期须重新连接。
- 资源观测：C盘约11.69GB空闲、D盘37.48GB、可用内存2.6GB；之后Go编译限制并行与堆，避免重复全量构建。
- Task 5 Ruling：官方目录加载器扫描组件根目录的全部子目录，测试目录被误识别为组件包；组件改放 integrations/langflow/components/teknowra，支持模块与测试留在上一层。只调整本轮新增文件，不动原型；需重启独立Langflow后验证无加载错误。
- 独立审查：一次全变更只读审查发现3项重要问题：运行中未重读员工/成员/Agent权限、多知识组件同名工具覆盖、MCP structuredContent丢失。已在同一修复轮补齐实时权限复核、知识库唯一工具名及底层结构化结果透传；两个知识工具路由覆盖先RED，修复后Python6项GREEN；StructuredOnly/NormalizeLangflow/LangflowLiveAuthority/LangflowCapabilities Go测试GREEN。未进行第二轮独立审批，不将主线程验证写成审查人再次批准。
- Task 5 实机：新合同信用图的模型、知识库、MCP下拉框成功从平台目录显示真实名称/ID，工具下拉列出find_customer/get_customer_credit_control/check_customer_access。连接刷新后清空，官方API读回两个新图的directory_connection/task_capability/bridge_url均为空；保存重载仍保留资源名称。证据general-dynamic-canvas.png。
- Task 6：已更新两个隔离测试图的知识组件唯一命名代码，保留可视资源选择；原442b5cb9原型图没有改动。正在重建本机后端以载入审查修复，真实聊天验收尚未完成。
- 接受的阶段限制：可编辑Flow/回环自动登录属于D/G；串行桥/缓冲输出属于E；多轮、技能、共享与多模态尚明确拒绝。本轮没有扩大到这些后续项。
- Task 6 实机知识摘要通过：会话612890e6-3564-4c66-ba62-0d55630ce8a9，trace864ebdeb27a34684ba53f2a3709ea2d3为ok，1个Agent、2次knowledge_search、没有CRM工具，最终答案2034字已保存。来源与金额/付款/政策事实正确，审批已完成状态保持未知。证据general-summary-evidence.json、general-summary-success.png。
- Task 6 首次新合同信用图失败：会话6f162fc4-37f4-4ae7-ac9c-cdfec684bc01，MCP组件解析运行时tools_json报Invalid control character。Langflow文本输入反转义了JSON里的描述；改为base64传输工具目录、Python严格解码，包含换行/引号/反斜杠的测试先RED后GREEN，共7项通过。新增Go实际Flow请求传输回归，待最终运行。一次正在运行的Go测试在修改import期间出现过失效依赖图错误，已停止，源码冻结后重新运行；不能记作通过。
- Task 6 双Agent修复后实机通过：会话90574b8e-8b7a-4378-9959-2214d0378938，trace726496791c7d490bb7d7ef741f6b2027为ok；2个Agent与5个原生工具span全部ok，实际客户ID/额度/评级与工具一致。沿用既有admin授权，不声称本轮重验新OAuth登录。答案已保存，证据general-business-evidence.json/general-business-success.png。
- Task 2 最终langflowruntime测试GREEN，包括实际请求的工具目录传输回归；service/router最终回归仍在执行。
- Closure：最终针对性Go回归完成，langflowruntime、application/service、router均GREEN；选择包含FlowWire、Langflow、NormalizeLangflow、Eino/Scope、APIKey/OAuth。之前MCP与tools的结构化/OAuth测试GREEN，前端20项和Python7项GREEN；diff --check为0。
- Closure：官方API最终读回两个已保存图，名称资源选择保留，directory_connection/task_capability/bridge_url均为空。合同信用7节点/7边，知识摘要5节点/4边；实际2/1 Agent两场景通过。任务1—6在本轮A+B+C范围完成；未运行全仓测试或全量前端生产构建，不声称D—G完成。
- 交付：integrations/langflow/README.md；研究目录的通用接入A-B-C验收记录.md与两份general-*-evidence.json。当前本机8080/7861已就绪，保留研发页签及服务便于用户查看；没有提交、推送或生产部署。最终C盘8.60GB、D盘37.14GB空闲；构建缓存/临时目录在D，C盘仍受Windows分页等系统使用影响。
