# 双引擎三项收口（已授权在当前工作区实施）

保留所有现有修改和历史数据；不迁移数据库、不生成交付包、不上线。仅本轮新增或改动的路径做针对性验证，不重跑已完成的业务全链。

1. 复用消息内部execution_context JSON留存受限诊断。两引擎采集事实复核拒绝、节点/任务/实现版本及安全错误分类；不保存原始提示词、完整草稿和凭据。普通消息/SSE不暴露；研发诊断接口仍须通过当前会话所有权与Contributor以上权限，API key/公开分享不能访问。
2. 后端提供引擎能力和Eino定义目录，页面不再维护独立场景枚举。保存前与执行端共享能力组合校验；不支持的配置分组隐藏并说明，防止保存后才发现不支持；保留历史数据，不静默改旧配置。
3. Eino已编译实现源和定义生成稳定SHA指纹，随任务步骤保存，定义ID与实现指纹分开。新任务记录当前实现，历史空值明确未知不补写；保留随后端版本回退的方式，不造独立发布系统。

验证：先失败用例再实现；消息持久化与诊断脱敏、接口权限/跨会话拒绝、保存校验与目录一致、实现指纹稳定/变化/历史保留；前端类型/SFC检查和真实页面配置与请求信息查看；仅必要的本机运行程序重建一次，不重复OAuth和完整多专家验收。

Ruling：复用当前dirty工作区，用户明确要求保留现有修改；不另建工作树、不提交。具体复核理由中可能含业务引用，仅授权研发在本人会话内查看，不把它作为普通回答。

## 进度

- 已核实持久化、会话所有权、RBAC、目录与引擎错误路径。尚未实施。

## 实施与审查记录

- 复用Message.ExecutionContext存诊断，整个字段仍json:"-"；现有错误事件/消息持久化负责写入。两引擎实际audit拒绝和无效结构在各自每次run的state采集，服务/初始化失败分类兜底，不增数据库表。普通JSON/SSE不携带诊断；新诊断端点要求web主体+Contributor以上，并调用原GetMessage会话所有权校验。
- 按原einodef目录提供场景/只读资源要求，页面不再硬编码四场景；保存与执行复用ValidateSelection。Eino单次任务禁用实际MultiTurnEnabled、HistoryTurns=0；默认逻辑不再给显式Eino强开多轮。该默认回填负例已观察RED后修正。历史Agent不批量改写，页面只在用户显式关闭不支持配置并保存后更新。
- Eino嵌入已编译运行时Go源和定义生成SHA指纹，任务步骤持久化ImplementationVersion；旧行空值保持未知，不回填。未知定义指纹负例已RED后修正。Langflow步骤复用既有固定图摘要。
- 前端vue-tsc已通过两次；初版相关6包定向检查已通过，审查后新增默认值/初始化失败兼容项的最终定向检查与构建仍进行中，未计为最终完成。
- 独立只读审查发现3项Important（实际多轮默认、旧Langflow设置无法关闭、初始化失败漏诊断）及空诊断覆盖兜底Minor，均已修复并复核收口。旧Langflow绑定也使用同一有效引擎判断；能力目录明确unsupported_settings。
- 当前未切换本机后端，仍旧canonical程序；真实页面配置/版本追溯验收尚待执行。没有上线、提交、推送或生成交付包。

## 最终定向检查

- 6个相关包的本轮定向用例全部通过：types、einoruntime、langflowruntime、handler、handler/session、application/service。覆盖诊断内部序列化/脱敏、真实模型桥保留拒绝响应、Eino拒绝不发布并带实现版本、角色/主体/会话所有权、失败处理及初始化兜底、资源组合与实际多轮默认开关。
- 另通过messageRepository真实Create/Update/Get的内存SQLite往返：内部诊断和实现版本保留，普通JSON不泄漏原因，另一条历史行不被回填。测试使用实际生成的消息UUID；初次测试因误用硬编码ID读不到行已修正，不是产品持久化故障，也未连接本机业务库。
- vue-tsc --noEmit通过，新增诊断弹层有高度上限。最终程序构建仍待结束；仅前述用例成功，不等于本轮界面验收已完成。

## 最终收口（本机验证通过）

- 构建成功，仅切换本机8080为 `WeKnora-dual-engine-followups.exe`，SHA256 `ff0c5554cf1f85cea4a58b107bec64c8da94331f19c3ecb7881a4f6ac5f2af31`。正常启动入口及candidate已更新；未开临时模型日志。原前端、Langflow、MCP、CRM进程不变。
- 真实页面使用原专用Agent `ae010dc1-0c04-421b-b428-46b468b3024a` 和会话 `449865ae-4941-41e3-8c1e-3f306a1a21c1`。旧多轮配置初始化拒绝，消息 `2d1ebfdd-2a0d-4c64-9933-76d806eb611a` 刷新后仍可见失败及内部分类；未调用模型或业务工具。通过页面明确关闭不支持的设置并保存，数据库确认multi_turn_enabled=false/history_turns=0，KB选择仍在。
- 随后仅一次合成合同只读核对完成四步骤，消息 `49edad7a-2936-4d50-8fe8-137214d3e4f9` 所有步骤实现版本均为 `eino-sha256:e98501f5047be8bc1609f2a4901e7d8768ffeb0b3753719b78f4d9290c5654f5`，与编辑目录一致，刷新仍可见。原历史任务显示未知不补写。新失败/成功消息均无knowledge_id。没有重复完整多专家、OAuth和双账号业务验收。
- 只读核验脚本 `research/upstream-runtime-20261017/verify_followups.local.py` exit0：34项源码哈希及程序SHA一致，选定旧历史哈希和原页面2条历史消息哈希不变，23个Agent保留，122/114均clean，MCP配置不变。报告 `dual-engine-followups-validation.local.json`；界面证据 `dual-followups-diagnostic.png`、`dual-followups-config.png`、`dual-followups-version.png`。candidate的本轮真实验收标记已置true。
- 已完成仅本轮三项；权限/事实复核无放宽，旧失败的原始理由未事后伪造。保留所有既有修改和历史数据；没有提交、推送、上线、镜像或交付包。
