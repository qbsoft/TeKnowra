import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

import { findEndSessionEndpoint, logoutIdentityProviderBeforeLogin } from './idpFreshLogin'

const read = (rel: string) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

const AUTHZ = 'https://id.yihe.work:9222/login/oauth/authorize?client_id=x&state=y'

type Call = { url: string; init?: RequestInit }

function fakeFetch(discovery: unknown, opts: { discoveryStatus?: number; hang?: boolean } = {}) {
  const calls: Call[] = []
  const impl = (async (url: string, init?: RequestInit) => {
    calls.push({ url, init })
    if (opts.hang) {
      return new Promise((_, reject) => init?.signal?.addEventListener('abort', () => reject(new Error('aborted'))))
    }
    if (url.endsWith('/.well-known/openid-configuration')) {
      return new Response(JSON.stringify(discovery), { status: opts.discoveryStatus ?? 200 })
    }
    return new Response(null, { status: 200 })
  }) as unknown as typeof fetch
  return { impl, calls }
}

test('按授权地址的源找发现文档，先退出再返回', async () => {
  const { impl, calls } = fakeFetch({ end_session_endpoint: 'https://id.yihe.work:9222/api/logout' })
  await logoutIdentityProviderBeforeLogin(AUTHZ, impl)
  assert.equal(calls.length, 2)
  assert.equal(calls[0].url, 'https://id.yihe.work:9222/.well-known/openid-configuration')
  assert.equal(calls[1].url, 'https://id.yihe.work:9222/api/logout')
  assert.equal(calls[1].init?.credentials, 'include', '要带上身份中心的登录 cookie，否则退不掉')
  assert.equal(calls[1].init?.mode, 'no-cors')
})

test('发现文档里没有退出地址：跳过，不拦登录', async () => {
  const { impl, calls } = fakeFetch({ issuer: 'https://id.yihe.work:9222' })
  await logoutIdentityProviderBeforeLogin(AUTHZ, impl)
  assert.equal(calls.length, 1)
})

test('退出地址指向别的主机：不去请求', async () => {
  const { impl } = fakeFetch({ end_session_endpoint: 'https://evil.example.com/logout' })
  assert.equal(await findEndSessionEndpoint(AUTHZ, impl), null)
})

test('发现文档取不到（issuer 带路径的身份中心等）：跳过', async () => {
  const { impl, calls } = fakeFetch({}, { discoveryStatus: 404 })
  await logoutIdentityProviderBeforeLogin(AUTHZ, impl)
  assert.equal(calls.length, 1)
})

test('身份中心不响应：超时后照样继续登录，不抛错', async () => {
  const { impl } = fakeFetch({}, { hang: true })
  const started = Date.now()
  await logoutIdentityProviderBeforeLogin(AUTHZ, impl, 50)
  assert.ok(Date.now() - started < 1000)
})

test('授权地址不合法：跳过', async () => {
  const { impl, calls } = fakeFetch({})
  await logoutIdentityProviderBeforeLogin('not a url', impl)
  assert.equal(calls.length, 0)
})

test('钩子还在：上游登录页跳转前先退出身份中心', () => {
  const src = read('../../views/auth/Login.vue')
  assert.match(src, /import \{ logoutIdentityProviderBeforeLogin \} from '@\/api\/auth\/idpFreshLogin'/)
  assert.match(src, /await logoutIdentityProviderBeforeLogin\(authorizationURL\)\s*\n\s*window\.location\.href = authorizationURL/)
})
