package session

import "strings"

// Keep raw provider, transport and credential-bearing errors out of answers.
func langflowFailureMessage(reason string) string {
	switch {
	case strings.HasPrefix(reason, "Langflow流程尚未接入或发布"):
		return "Langflow流程尚未接入或发布，请先在智能体页面创建或关联流程并发布版本。"
	case strings.HasPrefix(reason, "会话附件："):
		switch {
		case strings.Contains(reason, "超过完整审查输入上限"):
			return "会话附件超过完整审查输入上限（约12000 token、64KiB正文），请拆分后明确审查范围。"
		case strings.Contains(reason, "多份材料"):
			return "本会话有多份材料，请在本轮明确选择需要审查的附件。"
		case strings.Contains(reason, "OCR"):
			return "扫描件OCR尚不能确认覆盖全部页面，请上传可提取文本的合同。"
		case strings.Contains(reason, "过期"):
			return "会话附件已过期、被删除或不可访问，请重新上传。"
		case strings.Contains(reason, "尚未解析完成"):
			return "会话附件尚未解析完成或解析失败，请检查附件状态后重试。"
		default:
			return "会话附件校验未通过，请检查文件类型、正文及解析状态后重新选择附件。"
		}
	case strings.Contains(reason, "旧技能执行环境"):
		return "会话仍绑定旧技能执行环境，请新建会话后重试。"
	case strings.Contains(reason, "技能"):
		return "技能授权、版本或执行环境校验未通过，请检查技能是否启用及当前发布配置后重试。"
	case strings.Contains(reason, "授权") || strings.Contains(reason, "员工") || strings.Contains(reason, "共享"):
		return "当前身份或资源授权已失效，请检查共享和访问权限后重试。"
	default:
		return "执行服务或已发布配置校验未通过，请检查服务状态与发布配置后重试。"
	}
}
