import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

import {
  EMBED_HOST_USER_PLACEHOLDER,
  embedHostUserAttr,
  embedHostUserSuffix,
  forgetEmbedIdentityOnRequest,
  readEmbedHostUserFromLocation,
  setEmbedHostUser,
  withEmbedHostUserPlaceholder,
} from './hostUser'

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

// ── 平台生成的嵌入代码带上宿主用户 ─────────────────────────────────────────────
// 场景：别的系统照着「设置 → 网页嵌入」里平台给的代码贴进去。按人分开的能力必须从这个入口就能用上。

test('接入方没换占位符就贴上去：当作没传，不能让所有人落到同一个假用户名下', () => {
  setEmbedHostUser(EMBED_HOST_USER_PLACEHOLDER)
  assert.equal(embedHostUserSuffix(), '')
  setEmbedHostUser(`  ${EMBED_HOST_USER_PLACEHOLDER}  `)
  assert.equal(embedHostUserSuffix(), '')
})

test('iframe 方式：从地址里读宿主用户，# 后面优先', () => {
  assert.equal(readEmbedHostUserFromLocation({ hash: '#token=em_x&host_user=1867392610583715841' }), '1867392610583715841')
  assert.equal(readEmbedHostUserFromLocation({ hash: '#token=em_x', search: '?host_user=u2' }), 'u2')
  assert.equal(readEmbedHostUserFromLocation({ hash: '#host_user=a', search: '?host_user=b' }), 'a')
  assert.equal(readEmbedHostUserFromLocation({ hash: '#token=em_x' }), '')
  assert.equal(readEmbedHostUserFromLocation({ hash: '#host_user=%E5%BC%A0%E4%B8%89' }), '张三')
  assert.equal(readEmbedHostUserFromLocation(undefined), '')
  // 有人只把生成代码里的地址抠出来用（&amp; 没被还原）：照样读得出来，不能悄悄失效
  assert.equal(readEmbedHostUserFromLocation({ hash: '#token=em_x&amp;host_user=u-42' }), 'u-42')
})

test('生成的代码：浮窗多一行属性；iframe 地址接在 token 后面，没有 token 时自己起 #', () => {
  assert.equal(embedHostUserAttr(), 'data-host-user="CURRENT_USER_ID"')
  assert.equal(
    withEmbedHostUserPlaceholder('https://t.example.com/embed/ch#token=em_x'),
    'https://t.example.com/embed/ch#token=em_x&host_user=CURRENT_USER_ID',
  )
  assert.equal(
    withEmbedHostUserPlaceholder('https://t.example.com/embed/ch'),
    'https://t.example.com/embed/ch#host_user=CURRENT_USER_ID',
  )
  // 生成出来的地址，嵌入页自己要读得回来
  const url = new URL(withEmbedHostUserPlaceholder('https://t.example.com/embed/ch#token=em_x').replace('CURRENT_USER_ID', 'u-42'))
  assert.equal(readEmbedHostUserFromLocation({ hash: url.hash }), 'u-42')
})

// ── 退出登录时清除 ─────────────────────────────────────────────────────────
// 场景：admin 授权过经营问数后点了「退出登录」。编号留在浏览器里的话，之后会用开发者工具的人
// 能翻出它、借 admin 的授权取数。

const CH = 'd07fc028'
const visitorKey = (ch: string) => `weknora-embed-visitor:${ch}${embedHostUserSuffix()}`
const sessionKey = (ch: string) => `weknora-embed-session:${ch}${embedHostUserSuffix()}`

function withFakeStorage(initial: Record<string, string>, run: (store: Map<string, string>) => void) {
  const store = new Map(Object.entries(initial))
  const original = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: { removeItem: (k: string) => void store.delete(k), getItem: (k: string) => store.get(k) ?? null },
  })
  try {
    run(store)
  } finally {
    if (original) Object.defineProperty(globalThis, 'localStorage', original)
    else delete (globalThis as { localStorage?: unknown }).localStorage
  }
}

const forgetMsg = (channelId: string = CH) =>
  ({ data: { source: 'weknora-host', type: 'forget_identity', payload: { channel_id: channelId } } }) as MessageEvent
const trusted = () => true

test('退出登录：只删当前这个人的访客编号和对话指针，别人的、默认的都不碰', () => {
  setEmbedHostUser('admin-id')
  withFakeStorage(
    {
      [`weknora-embed-visitor:${CH}:u:admin-id`]: 'uuid-admin',
      [`weknora-embed-session:${CH}:u:admin-id`]: '{}',
      [`weknora-embed-visitor:${CH}:u:sales01-id`]: 'uuid-sales',
      [`weknora-embed-visitor:${CH}`]: 'uuid-default',
      'weknora-embed-visitor:other-channel:u:admin-id': 'uuid-other-channel',
      'weknora-embed-locale': 'zh-CN',
    },
    (store) => {
      assert.equal(forgetEmbedIdentityOnRequest(forgetMsg(), trusted, [visitorKey, sessionKey]), true)
      assert.deepEqual([...store.keys()].sort(), [
        'weknora-embed-locale',
        'weknora-embed-visitor:d07fc028',
        'weknora-embed-visitor:d07fc028:u:sales01-id',
        'weknora-embed-visitor:other-channel:u:admin-id',
      ])
    },
  )
})

test('不可信来源发来的清除消息：认领但什么都不删', () => {
  setEmbedHostUser('admin-id')
  withFakeStorage({ [`weknora-embed-visitor:${CH}:u:admin-id`]: 'uuid-admin' }, (store) => {
    assert.equal(forgetEmbedIdentityOnRequest(forgetMsg(), () => false, [visitorKey, sessionKey]), true)
    assert.equal(store.size, 1)
  })
})

test('没报过宿主用户的渠道（官网那种）：清除消息不删默认那一份', () => {
  setEmbedHostUser('')
  withFakeStorage({ [`weknora-embed-visitor:${CH}`]: 'uuid-default' }, (store) => {
    assert.equal(forgetEmbedIdentityOnRequest(forgetMsg(), trusted, [visitorKey, sessionKey]), true)
    assert.equal(store.size, 1)
  })
})

test('别的消息不归它管；没带渠道号的清除消息不删东西', () => {
  setEmbedHostUser('admin-id')
  withFakeStorage({ [`weknora-embed-visitor:${CH}:u:admin-id`]: 'uuid-admin' }, (store) => {
    const other = { data: { source: 'weknora-host', type: 'provide_token', token: 'x' } } as MessageEvent
    assert.equal(forgetEmbedIdentityOnRequest(other, trusted, [visitorKey, sessionKey]), false)
    assert.equal(forgetEmbedIdentityOnRequest(forgetMsg(''), trusted, [visitorKey, sessionKey]), true)
    assert.equal(store.size, 1)
  })
})

// 钩子挂在上游文件里，各一行。合并上游时若被冲掉，功能会悄悄退回「整个浏览器共用一份」
// ——不报错，但不同的人又会串用授权和对话。这条测试盯着它。
test('上游文件里的钩子都还在', () => {
  const api = read('./index.ts')
  assert.match(api, /EMBED_VISITOR_STORAGE_PREFIX\}\$\{channelId\}\$\{embedHostUserSuffix\(\)\}/)
  assert.match(api, /EMBED_CHAT_SESSION_STORAGE_PREFIX\}\$\{channelId\}\$\{embedHostUserSuffix\(\)\}/)
  // 必须先记下宿主用户，再交给 bootstrap —— 后者一进去就读访客编号
  assert.match(api, /setEmbedHostUser\(e\.data\.host_user\)\s*\n\s*handler\(token, e\.data\.channel_id\)/)

  const widget = read('../../../public/weknora-widget.js')
  assert.match(widget, /type: 'provide_token',[\s\S]{0,120}host_user: opts\.hostUser/)

  // 生成的嵌入代码带上宿主用户：三个 build*Snippet、嵌入页启动、浮窗脚本的自动初始化、设置面板里的说明
  assert.match(api, /escapeHtmlAttr\(withEmbedHostUserPlaceholder\(buildEmbedURL\(channelId, token\)\)\)/)
  assert.equal((api.match(/^\s+embedHostUserAttr\(\),$/gm) || []).length, 2, '浮窗和安全模式两种代码都要带这一行')
  const bridge = read('../../composables/useEmbedBridge.ts')
  const applyAt = bridge.indexOf('    applyEmbedHostUserFromLocation()')
  assert.ok(
    applyAt > 0 && applyAt < bridge.indexOf('await bootstrap(initialToken)'),
    '必须在 bootstrap 之前读地址里的宿主用户',
  )
  assert.match(widget, /hostUser: legacyScript\.getAttribute\('data-host-user'\),/)
  assert.match(read('../../components/AgentEmbedChannelPanel.vue'), /<EmbedHostUserHint :mode="drawerSnippetTab" \/>/)

  // 退出登录时清除：嵌入页在处理令牌消息之前先认领清除消息；小部件在实例和全局 API 上各暴露一个入口
  assert.match(api, /if \(forgetEmbedIdentityOnRequest\(e, isTrustedParentMessage, \[embedVisitorStorageKey, embedChatSessionStorageKey\]\)\) return/)
  assert.match(widget, /forgetIdentity: function \(\) \{ return postHostPayload\('forget_identity', \{ channel_id: channelId \}\); \}/)
  assert.match(widget, /forgetIdentity: function \(\) \{ return instance \? instance\.forgetIdentity\(\) : false; \}/)
})
