import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

import { normalizeAgentSourceTenant } from './agentOwnSource'

test('来源空间就是自己的空间：不算共享，返回 null（线上 admin 撞上的那种）', () => {
  assert.equal(normalizeAgentSourceTenant('10000', 10000), null)
  assert.equal(normalizeAgentSourceTenant(10000, '10000'), null)
  assert.equal(normalizeAgentSourceTenant(' 10000 ', '10000'), null)
})

test('来源是别的空间：原样保留，真正的共享不受影响', () => {
  assert.equal(normalizeAgentSourceTenant('10000', 10002), '10000')
  assert.equal(normalizeAgentSourceTenant(10000, '10002'), '10000')
})

test('没带来源、或还不知道当前空间：不猜', () => {
  assert.equal(normalizeAgentSourceTenant(null, 10000), null)
  assert.equal(normalizeAgentSourceTenant('', 10000), null)
  assert.equal(normalizeAgentSourceTenant('10000', ''), '10000')
  assert.equal(normalizeAgentSourceTenant('10000', undefined), '10000')
})

// 钩子挂在上游文件 stores/settings.ts 里。被合并冲掉的话，从共享空间选了自己智能体的人又会每次提问都 404。
test('settings store 的读取和写入两处都做了归一', () => {
  const src = readFileSync(fileURLToPath(new URL('./settings.ts', import.meta.url)), 'utf8')
  assert.match(src, /selectedAgentSourceTenantId: \(state\) =>\s*normalizeAgentSourceTenant\(state\.settings\.selectedAgentSourceTenantId, currentTenantIdFromStorage\(\)\)/)
  assert.match(src, /this\.settings\.selectedAgentSourceTenantId = normalizeAgentSourceTenant\(sourceTenantId, currentTenantIdFromStorage\(\)\)/)
})
