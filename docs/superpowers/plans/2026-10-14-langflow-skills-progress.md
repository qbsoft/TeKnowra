# SDD ledger — plan: D:/TeKnowra/repo/docs/superpowers/plans/2026-10-14-langflow-skills.md

- User authorization：继续下一轮技能接入，复用现有管理和权限并真实验收；沿用当前工作区实施，不再请求重复批准。
- Preflight：现有Langflow明确拒绝SkillsEnabled/SandboxConfigID；宿主QA已加载每轮TenantSkills。原生SkillManager提供正文/资源、ReadFileTool技能URI，ShellExecTool提供当前人员环境与会话执行。工作空间10000已有tyer-skill-selftest、tyer-contract-review两个ready安装，配置7fcb05cd-9d7a-4368-9c22-2a89622c9ff9；未读取或输出凭据。
- Ruling：技能元数据/资源在TeKnowra，脚本由宿主现有执行工具执行，Langflow只编排；技能脚本入口必须是授权技能内的文件，禁止自由shell字符串。
- Ruling：同一技能的requires_tools仅提示缺口，不授予新工具权限，复用平台原有语义。验收使用只读业务技能，不提交审批、付款或CRM回写。
- Tasks 1—5 pending；先测试后实现，最后唯一fresh reviewer。

- Tasks 1—3 implemented：Binding/目录/发布依赖、宿主原生SkillManager与受控ShellExec、严格/skill schema及Python可视组件均已落盘。针对性Go四包2026-10-14 23:23 PASS；Python技能组件2项PASS。技能节点/布尔脚本开关/未绑定pinned提示补充测试观察RED后GREEN。
- Task 4 in progress：通过平台技能管理上传并安装tyer-langflow-contract-evidence到原有自定义SKILL沙箱；Agent界面指定selftest与新业务技能。通过LF普通editor在画布添加两个技能节点、连Tools、配置脚本开关。后端正在重建，新组件已在LF界面出现。
- Script report tests：先缺少report.py RED，实现后2项GREEN（40000.00计算、30%阈值、非法金额/比例拒绝）。
- Baseline limitations：较宽Skill测试触发既有Windows权限/tar路径/Python环境不兼容失败；未改相关原生代码，不将宽测试集报告为通过。最终针对性命令为go test -p=1 -ldflags=-s -w ./internal/agent/langflowdef ./internal/agent/langflowrelease ./internal/agent/langflowruntime ./internal/application/service -run=TestSkillBindings|TestSkillBridge|TestPublishedSkill|TestSkillComponentProfile|TestSkillMetadata|TestLangflow|Test.*Eino -count=1。
- Task 4 evidence：V3 d8e7c967-84c2-43d9-a5a2-15b00865ddee私有运行工件870a5352-63a7-4999-86ce-b7011e265275；聊天beca45e5-2aae-4c3a-a733-091c798a1ee7实际selftest两次read_skill返回L2/L3；合同业务四次工具=read_skill正文、read_skill参考、knowledge_search、execute_skill_script，script输出40000.00，trace均ok，宿主正文已保存且带source_id。平台禁用业务技能后同聊天第三轮无业务答案、无新增LF trace；已用原管理UI重新启用。
- Final review：唯一fresh只读review /root/langflow_skills_review_20261014，Critical无，Important为旧session sandbox pin与发布绑定不符；进入一次TDD修复。Minor deferred：Go MCP别名未按Python规则截断/替换，长特殊名称提示兼容限制；技能资源未暴露分页，长资源不能续读。当前只读业务不受这两项影响，说明文档已明确。
- Ruling：直接当前工作区实施、不提交、不删除本轮审查材料和ledger — 用户明确要求保留所有未提交修改 — 若误解会增加人工整理成本，不能用自动清理技能指令覆盖用户要求。
- Final fixed：旧沙箱pin不符 — TestLangflowSkillsRejectOldSessionSandboxAfterAgentMoves RED（缺少校验）→GREEN，原生初始化后再次核对并每调用前后复查；四个协议/发布/目录/运行包完整测试PASS，service本轮Langflow/技能/Eino定向PASS，Python9+脚本2项PASS。
- Tasks 1—5 complete（工作区未提交，用户要求保留）：最终后端PID26828，启动23:49:29；原聊天23:50再次四次实际调用+40000.00，trace41407e95719a4272a5cd837475ec3b2b为ok，依然执行V3私有发布工件。持久答案与flow_id/release_id一致，恢复技能后正常；无二次review，依靠复现测试和最终实机复验。
- 本轮验收记录：D:/TeKnowra/research/selection/langflow-decision-test-20261013/技能接入验收记录.md；安全证据skills-runtime-evidence.json、skills-final-backend-artifact.json、skills-final-contract.png。保留已知空错误/图片占位UI问题，不宣称友好拒绝界面通过；不宣称项目全量Windows测试通过或生产完成。
