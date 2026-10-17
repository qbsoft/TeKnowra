// 对话里的 MCP 授权弹窗落脚页。TeKnowra 自己的文件。
//
// 授权完成后如果把弹窗送回正在进行的对话，那个地址会再开出一份对话，
// 原来挂着实时回答的页面却因为弹窗跨站之后被看成「已关闭」而停止轮询。
// v1 把弹窗送回站点根路径，对话页自己还在，所以能继续。
// 现在弹窗只落到这个短页：通知原来的页面，然后自己关掉。

export const MCP_OAUTH_DONE_PATH = '/mcp-oauth/done'
export const MCP_OAUTH_SIGNAL_KEY = 'weknora_mcp_oauth_signal'

// 弹窗跳到身份中心再跳回来时，打开它的页面会先看到 closed=true，令牌往往还没记上。
// 这段时间里不能停，否则授权成功了对话也不会往下走。
export const MCP_OAUTH_POPUP_GRACE_MS = 12000

// 同一条等待不能恢复两次。卡片被重新挂上时，新的询问和弹窗的询问会碰到一起。
const resumeClaimed = new Set<string>()

export function claimMcpOAuthResume(pendingId: string): boolean {
  if (!pendingId || resumeClaimed.has(pendingId)) return false
  resumeClaimed.add(pendingId)
  return true
}

export function releaseMcpOAuthResume(pendingId: string) {
  resumeClaimed.delete(pendingId)
}

export function mcpOAuthDoneUrl(): string {
  return MCP_OAUTH_DONE_PATH
}

export function mcpOAuthLanding(hash: string): 'success' | 'error' | null {
  const params = new URLSearchParams(hash.replace(/^#/, ''))
  if (params.get('mcp_oauth_result') === 'success') return 'success'
  if (params.has('mcp_oauth_error')) return 'error'
  return null
}

export function signalMcpOAuthLanding(result: 'success' | 'error') {
  localStorage.setItem(MCP_OAUTH_SIGNAL_KEY, JSON.stringify({ result, at: Date.now() }))
}

export function oauthPollGaveUp(closedForMs: number, graceMs = MCP_OAUTH_POPUP_GRACE_MS): boolean {
  return closedForMs >= graceMs
}

// 跨站登录时，打开弹窗的页面常常一开始就读到 closed=true，人还在登录页上。
// 这种「从没看见它打开过」不能当成用户关掉了。只有亲眼见过它开着、后来又关上，才开始宽限。
export function oauthPollShouldGiveUp(sawOpen: boolean, closedForMs: number, graceMs = MCP_OAUTH_POPUP_GRACE_MS): boolean {
  if (!sawOpen) return false
  return oauthPollGaveUp(closedForMs, graceMs)
}

export function popupLooksClosed(popup: Window | null): boolean {
  if (!popup) return true
  try {
    return popup.closed
  } catch {
    return false
  }
}

// 返回停止函数。授权成功时先停表，再执行 onAuthorized（里面会恢复被暂停的对话）。
// 弹窗看上去关了只通知 onGiveUp（把按钮放开），询问一直持续到成功、跳过或页面卸下。
export function watchMcpOAuthPopup(
  popup: Window | null,
  isAuthorized: () => Promise<boolean>,
  onAuthorized: () => Promise<void>,
  onGiveUp: () => void,
  graceMs = MCP_OAUTH_POPUP_GRACE_MS,
): () => void {
  let closedSince = 0
  let sawOpen = false
  let notifiedGone = false
  let stopped = false
  let checking = false
  let again = false

  const stop = () => {
    if (stopped) return
    stopped = true
    window.clearInterval(timer)
    window.removeEventListener('storage', onStorage)
  }

  const tick = async () => {
    if (stopped) return
    if (checking) {
      again = true
      return
    }
    checking = true
    try {
      if (popupLooksClosed(popup)) {
        if (sawOpen && !closedSince) closedSince = Date.now()
      } else {
        sawOpen = true
        closedSince = 0
      }
      let ok = false
      try {
        ok = await isAuthorized()
      } catch {
        ok = false
      }
      if (stopped) return
      if (ok) {
        stop()
        try { popup?.close() } catch { /* 跨站窗口关不掉 */ }
        await onAuthorized()
        return
      }
      // 跨站登录时弹窗经常一开始就像关掉了，人还在登录页上。这时只把按钮放开，
      // 询问不能停：令牌往往要等登录完成才记上，停了对话就不会往下走。
      if (!notifiedGone && oauthPollShouldGiveUp(sawOpen, closedSince ? Date.now() - closedSince : 0, graceMs)) {
        notifiedGone = true
        onGiveUp()
      }
    } finally {
      checking = false
      if (again && !stopped) {
        again = false
        void tick()
      }
    }
  }

  const onStorage = (event: StorageEvent) => {
    if (event.key === MCP_OAUTH_SIGNAL_KEY) void tick()
  }
  window.addEventListener('storage', onStorage)
  const timer = window.setInterval(() => { void tick() }, 1500)
  void tick()
  return stop
}
