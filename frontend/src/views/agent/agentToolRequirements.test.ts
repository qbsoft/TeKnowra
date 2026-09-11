import assert from 'node:assert/strict'
import test from 'node:test'

import {
  callableToolNames,
  canResolveRequirements,
  enabledSkillNames,
  mcpCandidateServiceIds,
  unmetSkillTools,
} from './agentToolRequirements'

const SERVICES = [
  { id: 'a', enabled: true },
  { id: 'b', enabled: true },
  { id: 'c', enabled: false },
]

test('mode=all 取空间里所有启用的服务，而不是 config.mcp_services', () => {
  // 这条最容易做错：mode=all 时 config.mcp_services 本来就是空的，
  // 照着它算会得出「一个 MCP 工具都没有」。
  assert.deepEqual(mcpCandidateServiceIds('all', [], SERVICES), ['a', 'b'])
  assert.deepEqual(mcpCandidateServiceIds('', [], SERVICES), ['a', 'b'], '未设置等同 all')
})

test('mode=selected 只取勾中的，mode=none 一个都不取', () => {
  assert.deepEqual(mcpCandidateServiceIds('selected', ['b'], SERVICES), ['b'])
  assert.deepEqual(mcpCandidateServiceIds('none', ['b'], SERVICES), [])
})

// 授权键格式 mcp:<服务ID>:<工具名>，由后端 tools.AgentMCPToolKey 生成，
// 前端只透传。旧的注册名（mcp_mail_send_email）已弃用。
const MAIL_SVC = '6a1e8a53-0000-4000-8000-000000000001'
const MAIL = [
  { tool_name: 'send_email', authorization_key: `mcp:${MAIL_SVC}:send_email` },
  { tool_name: 'selftest', authorization_key: `mcp:${MAIL_SVC}:selftest` },
]

test('按工具授权时，只有勾中的那条的原名才算能调', () => {
  const callable = callableToolNames(
    ['thinking', `mcp:${MAIL_SVC}:send_email`],
    { mail: MAIL },
  )
  assert.ok(callable.has('send_email'), '勾了 send_email，技能写原名要能对上')
  assert.ok(callable.has(`mcp:${MAIL_SVC}:send_email`))
  assert.ok(!callable.has('selftest'), '没勾的工具不能算数')
})

test('名单没写的工具就是不能用，没有「未配置」这种例外', () => {
  // 曾经有过一条兼容规则：名单里一个 MCP 授权都没有时视为「按工具授权之前
  // 存的」，于是候选服务的工具全部放行。它只为保住已部署的 agent 而存在，
  // 代价是配置不诚实——编辑器显示六个没勾，agent 却六个都在调。已去掉。
  const callable = callableToolNames(['thinking'], { mail: MAIL })
  assert.ok(!callable.has('send_email'), '没点名就不能用')
  assert.ok(!callable.has('selftest'))

  // 旧格式的注册名同样不算数——它解析不回服务 ID，授权不了任何东西。
  const legacy = callableToolNames(['mcp_mail_send_email'], { mail: MAIL })
  assert.ok(!legacy.has('send_email'), '旧注册名不能再当授权用')
})

test('不做后缀匹配', () => {
  // 'email' 是 'send_email' 的后缀。放过它的话，真缺的工具会被判成有，
  // 这个提示就正好在该响的时候不响。
  const callable = callableToolNames([`mcp:${MAIL_SVC}:send_email`], { mail: MAIL })
  assert.ok(!callable.has('email'))
})

const SKILLS = [
  { name: 'contract-review', requires_tools: ['list_review_templates', 'send_email'] },
  { name: 'plain' },
]

test('报出启用技能里没被授予的工具', () => {
  const callable = callableToolNames([`mcp:${MAIL_SVC}:send_email`], { mail: MAIL })
  const gaps = unmetSkillTools(SKILLS, new Set(['contract-review']), callable)
  assert.deepEqual(gaps, { 'contract-review': ['list_review_templates'] })
})

test('没启用的技能不报', () => {
  const callable = callableToolNames([], {})
  assert.deepEqual(unmetSkillTools(SKILLS, new Set(), callable), {})
})

test('没声明 requires_tools 的技能永远不报', () => {
  // 没声明 ≠ 不需要工具，多数技能是这个字段出现之前写的。
  const gaps = unmetSkillTools(SKILLS, new Set(['plain']), new Set())
  assert.deepEqual(gaps, {})
})

test('技能模式决定谁被启用', () => {
  assert.deepEqual([...enabledSkillNames('all', SKILLS, [])], ['contract-review', 'plain'])
  assert.deepEqual([...enabledSkillNames('selected', SKILLS, ['plain'])], ['plain'])
  assert.deepEqual([...enabledSkillNames('none', SKILLS, ['plain'])], [])
})

test('所有候选服务都拉到了才能给结论', () => {
  const ok = () => 'ok' as const
  assert.equal(canResolveRequirements(['a', 'b'], ok), true)
  assert.equal(canResolveRequirements([], ok), true, '没有候选服务时不存在映射缺口')
})

test('有服务拉不到就不给结论', () => {
  // 回归：MCP 服务器挂了的时候，get_review_summary 明明勾着，却被报成缺失。
  // 拿不到工具列表就拿不到 send_email ↔ mcp_mail_send_email 的映射，
  // 这时候说「缺」是误报，而误报几次这个提示就没人看了。
  assert.equal(canResolveRequirements(['a', 'b'], id => (id === 'b' ? 'error' : 'ok')), false)
  assert.equal(canResolveRequirements(['a'], () => 'loading'), false)
})
