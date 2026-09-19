import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

import { embedOAuthLandingMessage } from './oauthLanding'

test('授权成功的落脚页给出成功提示', () => {
  assert.match(embedOAuthLandingMessage('#mcp_oauth_result=success') || '', /授权成功|Authorized/)
})

test('授权失败的落脚页给出重试提示，且不冒充成功', () => {
  const msg = embedOAuthLandingMessage('#mcp_oauth_error=authorization_failed') || ''
  assert.match(msg, /没有完成|not completed/)
  assert.doesNotMatch(msg, /授权成功|Authorized\./)
})

test('普通嵌入页不受影响', () => {
  for (const h of ['', '#', '#token=em_xxx', '#mcp_oauth_result=pending', '#foo=mcp_oauth_result']) {
    assert.equal(embedOAuthLandingMessage(h), null, h)
  }
})

// 钩子在上游文件里只有一行，合并上游时若被冲掉，弹窗又会显示那句像报错的话。
test('useEmbedBridge.start() 里的钩子还在，且在任何报错分支之前', () => {
  const src = readFileSync(fileURLToPath(new URL('../../composables/useEmbedBridge.ts', import.meta.url)), 'utf8')
  const start = src.indexOf('const start = async () => {')
  assert.ok(start > 0, 'start() not found (renamed upstream?)')
  const body = src.slice(start)
  const hook = body.indexOf('handleEmbedOAuthLanding(')
  assert.ok(hook > 0, 'landing hook missing')
  assert.ok(hook < body.indexOf("t('embedPublish.missingChannel')"), 'hook must run before the missing-channel error')
})
