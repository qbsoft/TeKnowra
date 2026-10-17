import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

import { mcpOAuthLanding, oauthPollGaveUp, oauthPollShouldGiveUp, watchMcpOAuthPopup } from './mcpOAuthDone'

test('授权成功和失败的落脚标记能认出来', () => {
  assert.equal(mcpOAuthLanding('#mcp_oauth_result=success'), 'success')
  assert.equal(mcpOAuthLanding('#mcp_oauth_error=authorization_failed'), 'error')
  for (const hash of ['', '#', '#oidc_result=abc', '#mcp_oauth_result=pending']) {
    assert.equal(mcpOAuthLanding(hash), null, hash)
  }
})

test('弹窗刚被看成关闭时还要继续问，宽限过了才停', () => {
  assert.equal(oauthPollGaveUp(0), false)
  assert.equal(oauthPollGaveUp(11999), false)
  assert.equal(oauthPollGaveUp(12000), true)
  assert.equal(oauthPollShouldGiveUp(false, 12000), false)
  assert.equal(oauthPollShouldGiveUp(true, 11999), false)
  assert.equal(oauthPollShouldGiveUp(true, 12000), true)
})

test('弹窗被看成关掉之后仍继续问，授权完成后才往下走', async () => {
  const popup = { closed: false, close() { this.closed = true } }
  let intervalFn: (() => void) | null = null
  const previousWindow = globalThis.window
  globalThis.window = {
    setInterval(fn: () => void) {
      intervalFn = fn
      return 1
    },
    clearInterval() {
      intervalFn = null
    },
    addEventListener() {},
    removeEventListener() {},
  } as unknown as Window & typeof globalThis
  const originalNow = Date.now
  let now = 1_000_000
  Date.now = () => now
  let authorized = false
  let resumed = false
  let gaveUp = false
  const flush = () => new Promise((resolve) => setImmediate(resolve))
  try {
    const stop = watchMcpOAuthPopup(
      popup as unknown as Window,
      async () => authorized,
      async () => { resumed = true },
      () => { gaveUp = true },
    )
    await flush()
    popup.closed = true
    ;(intervalFn as (() => void) | null)?.()
    await flush()
    now += 12_000
    ;(intervalFn as (() => void) | null)?.()
    await flush()
    assert.equal(gaveUp, true)
    assert.equal(resumed, false)
    assert.notEqual(intervalFn, null)
    authorized = true
    ;(intervalFn as (() => void) | null)?.()
    await flush()
    assert.equal(resumed, true)
    stop()
  } finally {
    Date.now = originalNow
    globalThis.window = previousWindow
  }
})

test('对话授权不再把弹窗送回对话页本身', () => {
  const src = readFileSync(
    fileURLToPath(new URL('../views/chat/components/McpOAuthCard.vue', import.meta.url)),
    'utf8',
  )
  assert.match(src, /mcpOAuthDoneUrl\(/)
  assert.match(src, /watchMcpOAuthPopup\(/)
  assert.match(src, /resumeIfTokenReady\(/)
  assert.doesNotMatch(src, /window\.location\.origin \+ '\/'/)
})
