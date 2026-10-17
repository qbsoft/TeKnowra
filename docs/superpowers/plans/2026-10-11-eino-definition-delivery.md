# 统一定义接入实施计划

依据：对应日期的 definition-delivery-design.md。按用户持续推进要求在当前会话直接实施；不另开研发任务、不提交、不上线。

1. 先增加第三定义绑定测试与执行测试，运行并确认当前因 unsupported definition 拒绝。
2. 新增 internal/agent/einodef 编译期目录，Lookup 返回独立的能力描述副本；ValidateConfig 根据知识或 OAuth 工具要求验证配置。迁移配置加载和聊天授权范围选择。
3. 新增运行时 publication 目录，Definition 包含 Build 和 BeforeOutput；New 校验目录能力，Execute 使用构建器和业务复核回调。新增第三个知识摘要业务模块。
4. 服务按能力装配知识检索或 OAuth 只读回调。信用工具名来自发布描述，不取消本人身份、服务状态、工具目录、参数及授权卡检查。保留旧校验函数作为兼容包装供原测试调用。
5. 验证第三定义执行及权限变化拒绝；验证目录覆盖、配置安全限制、原合同/信用测试。运行 go test ./internal/agent/einodef ./internal/agent/einoruntime ./internal/config ./internal/application/service，编译 ./cmd/server。结果记录到研究报告，不宣称浏览器发布完成。

Review Focus：未知定义不能静默变合同；描述副本不能改全局工具权限；目录与构建器一致；业务输出钩子不能替代平台知识复核；新工具不能绕过现有 OAuth gate。

## 执行记录

- 五项接入实施步骤完成，本地第三定义测试 RED→GREEN；独立审查的事实复核问题也经 RED→GREEN 修补。
- 目录/运行时/配置包全量通过，服务和 OAuth 工具针对性通过，后端编译通过。
- 服务全量仍失败，含 SQLite 清理、跨平台技能环境和检索日志断言；没有完成基线对比，不擅自归因全部为环境问题。
- 未提交、未上线、未改本机绑定，正式浏览器发布与 ADK 接入仍待后续实施。阶段结果在 research/selection/eino-delivery-cost-20261011/统一接入阶段结果.md。
