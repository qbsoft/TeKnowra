import { get } from "../../utils/request";

// Skill信息
export interface SkillInfo {
  name: string;
  description: string;
  // 该技能正文里点名调用的工具，用工具自己的名字（send_email），不是注册表里
  // 带服务前缀的 mcp_mail_send_email —— 前缀取决于本空间把服务叫什么，技能
  // 文件里写不了。多数技能没声明，此时字段不存在，表示作者没提出主张，
  // 而不是「不需要任何工具」。
  requires_tools?: string[];
}

// 获取预装Skills列表；skills_available 为 false 表示沙箱未启用，前端应隐藏/禁用 Skills 配置
export function listSkills() {
  return get<{ data: SkillInfo[]; skills_available?: boolean }>('/api/v1/skills');
}
