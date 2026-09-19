import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

import { embedHostUserSuffix, setEmbedHostUser } from './hostUser'

const read = (rel: string) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

test('没传宿主用户：后缀为空，存储键与上游逐字节一致', () => {
  setEmbedHostUser(undefined)
  assert.equal(embedHostUserSuffix(), '')
  for (const v of ['', '   ', null, {}, [], true]) {
    setEmbedHostUser(v)
    assert.equal(embedHostUserSuffix(), '', `value ${JSON.stringify(v)} must count as absent`)
  }
})

test('不同的人得到不同后缀，换回来得到原来的后缀', () => {
  setEmbedHostUser('1867392610583715841')
  const manager = embedHostUserSuffix()
  setEmbedHostUser('1867392610583715999')
  const sales = embedHostUserSuffix()
  assert.notEqual(manager, sales)
  setEmbedHostUser('1867392610583715841')
  assert.equal(embedHostUserSuffix(), manager)
})

test('19 位雪花 ID 按字符串原样保留，不丢精度', () => {
  setEmbedHostUser('1867392610583715841')
  assert.equal(embedHostUserSuffix(), ':u:1867392610583715841')
})

test('换人后必须清掉上一个人：传空就回到默认那一套', () => {
  setEmbedHostUser('alice')
  setEmbedHostUser('')
  assert.equal(embedHostUserSuffix(), '')
})

test('特殊字符被转义、超长被截断，拼不出别人的键', () => {
  setEmbedHostUser('a:u:b/../c')
  assert.equal(embedHostUserSuffix(), ':u:a%3Au%3Ab%2F..%2Fc')
  setEmbedHostUser('x'.repeat(500))
  assert.equal(embedHostUserSuffix().length, ':u:'.length + 128)
})

// 钩子挂在上游文件里，各一行。合并上游时若被冲掉，功能会悄悄退回「整个浏览器共用一份」
// ——不报错，但不同的人又会串用授权和对话。这条测试盯着它。
test('上游文件里的 4 个钩子都还在', () => {
  const api = read('./index.ts')
  assert.match(api, /EMBED_VISITOR_STORAGE_PREFIX\}\$\{channelId\}\$\{embedHostUserSuffix\(\)\}/)
  assert.match(api, /EMBED_CHAT_SESSION_STORAGE_PREFIX\}\$\{channelId\}\$\{embedHostUserSuffix\(\)\}/)
  // 必须先记下宿主用户，再交给 bootstrap —— 后者一进去就读访客编号
  assert.match(api, /setEmbedHostUser\(e\.data\.host_user\)\s*\n\s*handler\(token, e\.data\.channel_id\)/)

  const widget = read('../../../public/weknora-widget.js')
  assert.match(widget, /type: 'provide_token',[\s\S]{0,120}host_user: opts\.hostUser/)
})
