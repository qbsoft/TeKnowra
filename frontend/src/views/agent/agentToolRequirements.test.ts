import assert from 'node:assert/strict'
import test from 'node:test'

import {
  callableToolNames,
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

const MAIL = [
  { tool_name: 'send_email', registry_name: 'mcp_mail_send_email' },
  { tool_name: 'selftest', registry_name: 'mcp_mail_selftest' },
]

test('按工具授权时，只有勾中的那条的原名才算能调', () => {
  const callable = callableToolNames(
    ['thinking', 'mcp_mail_send_email'],
    { mail: MAIL },
  )
  assert.ok(callable.has('send_email'), '勾了 send_email，技能写原名要能对上')
  assert.ok(callable.has('mcp_mail_send_email'))
  assert.ok(!callable.has('selftest'), '没勾的工具不能算数')
})

test('名单里一个 mcp_ 都没有时，候选服务的工具全部算能调', () => {
  // 对齐后端 applyMCPToolAllowlist：这种名单是按工具授权之前存的，后端不筛。
  const callable = callableToolNames(['thinking'], { mail: MAIL })
  assert.ok(callable.has('send_email'))
  assert.ok(callable.has('selftest'))
})

test('不做后缀匹配', () => {
  // 'email' 是 'mcp_mail_send_email' 的后缀。放过它的话，真缺的工具会被判成
  // 有，这个提示就正好在该响的时候不响。
  const callable = callableToolNames(['mcp_mail_send_email'], { mail: MAIL })
  assert.ok(!callable.has('email'))
})

const SKILLS = [
  { name: 'contract-review', requires_tools: ['list_review_templates', 'send_email'] },
  { name: 'plain' },
]

test('报出启用技能里没被授予的工具', () => {
  const callable = callableToolNames(['mcp_mail_send_email'], { mail: MAIL })
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
