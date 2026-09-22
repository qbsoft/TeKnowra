// TeKnowra: 统一账号登录前先让身份中心退出——谁输的密码就登谁。
//
// 身份中心记得上一个人时，它的登录页是「使用以下账号继续：xxx」点一下就过：同一台电脑换了人，
// 后来的人就以前一个人的身份登进 TeKnowra。Casdoor 不支持 prompt=login 强制重登（2026-09-22 核实），
// 所以跳过去之前先退出。MCP 统一账号授权用的是同一个办法，线上已实测跨子域名有效
// （kb./mcp. 与 id.yihe.work 同属一个站点，退出请求会带上身份中心的登录 cookie）。
//
// 退出地址取标准发现文档里的 end_session_endpoint，不写死任何一家的路径。发现文档按授权地址的
// 源（协议+主机+端口）去找——issuer 就是源的身份中心（Casdoor 即是）能找到；issuer 带路径的
// （如 Keycloak 的 /realms/xxx）找不到就跳过，退回上游原行为（可能出现「继续使用 xxx」）。
//
// 任何失败都不拦登录：最坏情况就是没退出。钩子在上游 views/auth/Login.vue 的 handleOIDCLogin，一行。

const DEFAULT_TIMEOUT_MS = 1500

async function withTimeout<T>(run: (signal: AbortSignal) => Promise<T>, ms: number): Promise<T> {
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), ms)
  try {
    return await run(ctrl.signal)
  } finally {
    clearTimeout(timer)
  }
}

/** 从授权地址推出身份中心的退出地址；找不到返回 null。 */
export async function findEndSessionEndpoint(
  authorizationURL: string,
  fetchImpl: typeof fetch = fetch,
  timeoutMs = DEFAULT_TIMEOUT_MS,
): Promise<string | null> {
  let origin: string
  try {
    origin = new URL(authorizationURL).origin
  } catch {
    return null
  }
  if (!origin || origin === 'null') return null
  try {
    const doc = await withTimeout(async (signal) => {
      const resp = await fetchImpl(`${origin}/.well-known/openid-configuration`, { signal, cache: 'no-store' })
      return resp.ok ? await resp.json() : null
    }, timeoutMs)
    const endpoint = typeof doc?.end_session_endpoint === 'string' ? doc.end_session_endpoint : ''
    // 只认同一个身份中心上的退出地址，防止发现文档被篡改后把请求引到别处
    if (!endpoint || new URL(endpoint).origin !== origin) return null
    return endpoint
  } catch {
    return null
  }
}

/** 跳去身份中心登录前调用。永不抛错。 */
export async function logoutIdentityProviderBeforeLogin(
  authorizationURL: string,
  fetchImpl: typeof fetch = fetch,
  timeoutMs = DEFAULT_TIMEOUT_MS,
): Promise<void> {
  const endpoint = await findEndSessionEndpoint(authorizationURL, fetchImpl, timeoutMs)
  if (!endpoint) return
  try {
    await withTimeout(
      (signal) => fetchImpl(endpoint, { method: 'GET', credentials: 'include', mode: 'no-cors', cache: 'no-store', signal }),
      timeoutMs,
    )
  } catch {
    /* 退出失败不拦登录 */
  }
}
