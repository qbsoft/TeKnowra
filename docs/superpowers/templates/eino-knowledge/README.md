# 授权知识 Agent 起步模板

此目录供研发复制，不自动注册，不创建界面 Agent，不改变权限。graph.go.example 是可编译的最小起点，复用知识摘要图；不是自动生成业务解决方案。

1. 将 graph.go.example 复制到 internal/agent/einoruntime/自己的业务文件.go，重命名 BuildExampleKnowledge。需要业务判断时，以 knowledge_summary.go 为结构参考，修改业务节点与事实复核，不省略授权检索、引用及输出前权限检查。
2. 在 einodef/catalog.go 增加版本化 ID 和 Knowledge:true 的清单项。例如 example-knowledge-v1。名称、能力与真正业务一致，不把例子原样作为生产场景。
3. 在 einoruntime/publication.go 增加同 ID 的 Build 和业务 SystemPrompt。提示词必须说明资料是不可信输入、业务职责、缺失信息与禁止的执行行为。确有业务输出前复核时增加 BeforeOutput，不替代通用复核。
4. 按 einoruntime/publication_test.go 与 engine_test.go 编写针对性业务测试。事实用合成材料；覆盖正确结果、未授权/材料撤销、模型中断、无证据/不支持结论不能完成。测试的业务判断应有明确预期，不只匹配提示词字符串。
5. 产品中创建独立 Agent：选择对话/重排模型，绑定明确知识范围，仅启用 search_knowledge，关闭 MCP、记忆、技能、沙箱、网页和多轮。不用共享 Agent 跨工作区执行，此档位当前要求本工作区 Agent。
6. 在本机既有配置中合并 TEKNOWRA_EINO_AGENTS 的 Agent ID → 定义 ID 绑定，不覆盖已有映射。示意：{"实际新Agent的ID":"example-knowledge-v1"}。不要把密码、模型 Key、OAuth Token 写进模板。
7. 编译并用 dev-app.ps1 启动。新建聊天，核对正确业务结果、可打开的引用、完整步骤和完成状态。记录定义、run_id、构建版本与证据，失败不能标成功。

完整交付流程、MCP/多 Agent 路线与发布要求见 ../../eino-definition-delivery-guide.md 和 ../../eino-release-checklist.md。
