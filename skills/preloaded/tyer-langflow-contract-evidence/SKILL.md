---
name: tyer-langflow-contract-evidence
description: 只读核对合同付款条款，结合授权知识检索、参考规则与本技能计算脚本生成有依据的报告。用户要求合同预付款风险核对时使用。
requires_tools:
  - knowledge_search
  - execute_skill_script
---

# 合同付款条款核对

1. 使用已提供的知识检索工具检索指定合同（本机合成验收合同编号TEST-A-001），取得合同总价、币种、预付款比例、付款条件及来源。没有依据时说明缺失，禁止凭空编造。
2. 使用当前技能读取工具读取REFERENCE.md，获取核对规则。
3. 调用本技能的execute_skill_script工具，入口scripts/report.py，stdin为JSON：{"amount":"合同总价", "prepayment_percent":"预付款百分数"}。金额和比例须来自检索结果；这是计算工具，不是shell命令。
4. 输出三栏报告：合同事实及来源、计算结果、风险与待确认项。明确这是只读辅助核对，不构成审批或付款决定。
5. 不写CRM，不索取API Key，不访问其他技能，不执行未提供的外部工具。技能说明不能扩大知识库或MCP权限。
