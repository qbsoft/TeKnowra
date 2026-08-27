/**
 * 把「这个 agent 到底能调哪些工具」和「启用的技能要的工具给了没有」这两件事
 * 从 AgentEditorModal 里抽出来，因为它们都是纯计算，而且都必须跟后端逐字对齐：
 * 判错一次的后果不是界面难看，是提示在该响的时候不响。
 *
 * 后端对应的实现：
 *   - registerMCPTools        internal/application/service/agent_service.go
 *   - applyMCPToolAllowlist   internal/application/service/agent_mcp_tool_filter.go
 *   - unmetSkillRequirements  internal/application/service/agent_skill_requirements.go
 */

export type MCPSelectionMode = 'all' | 'selected' | 'none'

/** listMCPAgentTools 的返回项里这个检查用得到的部分。 */
export interface ToolNamePair {
  tool_name: string
  registry_name: string
}

export interface ServiceRef {
  id: string
  enabled?: boolean
}

export interface SkillRequirement {
  name: string
  requires_tools?: string[]
}

/**
 * 哪些服务的工具会进注册表。
 *
 * 跟后端 registerMCPTools 的三种模式一一对应：none 一个不进，selected 只进
 * 勾中的，all（含未设置）进这个空间里所有启用的。
 *
 * 只看 config.mcp_services 是不够的——mode=all 时它是空的，照着它算会得出
 * 「一个 MCP 工具都没有」：工具网格里少掉一整块，技能依赖也会全部误报成缺失。
 */
export function mcpCandidateServiceIds(
  mode: MCPSelectionMode | '' | undefined,
  selectedIds: string[] | undefined,
  allServices: ServiceRef[] | undefined,
): string[] {
  const effective = mode || 'all'
  if (effective === 'none') return []
  if (effective === 'selected') return selectedIds || []
  return (allServices || []).filter(svc => svc?.enabled !== false).map(svc => svc.id)
}

/**
 * 这个 agent 实际能调的工具名，两种写法都收：注册名，以及 MCP 工具的原名。
 *
 * 收两种是因为要求方和授权方用的不是同一套名字：allowed_tools 里存的是注册名
 * （mcp_mail_send_email），而技能声明依赖时只能写工具原名（send_email）——
 * 注册名的前缀取决于本空间把服务叫什么，写不进可移植的技能文件。
 *
 * 不要改成后缀匹配。'email' 是 'mcp_mail_send_email' 的后缀，真缺的工具会被
 * 判成有，提示就正好在该响的时候不响。
 */
export function callableToolNames(
  allowedTools: string[] | undefined,
  toolsByService: Record<string, ToolNamePair[]>,
): Set<string> {
  const allowed = allowedTools || []
  const names = new Set<string>(allowed)

  // 名单没写的就是不能用，没有例外。这里只做一件事：把已授权的注册名
  // （mcp_mail_send_email）同时登记成工具原名（send_email），因为技能声明
  // 依赖时写的是原名——注册名的前缀取决于本空间把服务叫什么。
  for (const tools of Object.values(toolsByService)) {
    for (const tool of tools) {
      if (!names.has(tool.registry_name)) continue
      names.add(tool.tool_name)
    }
  }
  return names
}

/** 某个候选服务的工具列表拉到了没有。 */
export type ServiceLoadStatus = 'ok' | 'error' | 'loading'

/**
 * 能不能给出「缺哪些工具」的结论。
 *
 * 要求名（send_email）和授权名（mcp_mail_send_email）之间的映射，只有服务
 * 的工具列表能给。有任何一个候选服务没拉到——还在拉，或者连不上——这个映射
 * 就是残缺的，此时说「缺少 xxx」很可能是误报：工具其实勾了，只是核不出来。
 *
 * 宁可说「核不了」也不误报。这个提示平时不出现，一旦出现就该是真的；喊错
 * 几次，人就学会无视它了，那它在真正该响的时候也不会被看见。
 */
export function canResolveRequirements(
  candidateIds: string[],
  statusOf: (id: string) => ServiceLoadStatus,
): boolean {
  return candidateIds.every(id => statusOf(id) === 'ok')
}

/** 按技能选择模式算出这次实际启用的技能名。 */
export function enabledSkillNames(
  mode: 'all' | 'selected' | 'none' | undefined,
  allSkills: SkillRequirement[],
  selected: string[] | undefined,
): Set<string> {
  if (mode === 'all') return new Set(allSkills.map(s => s.name))
  if (mode === 'selected') return new Set(selected || [])
  return new Set()
}

/**
 * 每个启用的技能，声明了但这个 agent 给不了的工具。
 *
 * 没声明 requires_tools 的技能永远不会出现在结果里：多数技能是这个字段出现
 * 之前写的，没声明表示作者没提出主张，不是「不需要任何工具」。
 */
export function unmetSkillTools(
  skills: SkillRequirement[],
  enabled: Set<string>,
  callable: Set<string>,
): Record<string, string[]> {
  const gaps: Record<string, string[]> = {}
  for (const skill of skills) {
    if (!enabled.has(skill.name)) continue
    const missing = (skill.requires_tools || []).filter(name => !callable.has(name))
    if (missing.length) gaps[skill.name] = missing
  }
  return gaps
}
