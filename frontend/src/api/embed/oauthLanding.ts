// 嵌入页授权弹窗的落脚页 —— TeKnowra fork 自己的文件，上游没有。
//
// 嵌入访客点「去授权」会弹出一个窗口走 OAuth；走完后平台把弹窗重定向回嵌入页地址，
// 末尾带 #mcp_oauth_result=success 或 #mcp_oauth_error=...。这个地址不带渠道 token，
// 上游的嵌入页于是显示「缺少嵌入渠道或 Token」——授权明明成功了，看上去却像报错。
// （原来那个浮窗里的授权卡片自己会轮询到结果，不依赖这个弹窗。）
//
// 这里只负责认出这种落脚，给一句人话，并尝试把弹窗关掉。
// 钩子：composables/useEmbedBridge.ts 的 start() 开头一行。被冲掉的话 oauthLanding.test.ts 会红。

const zh = () => typeof navigator === 'undefined' || !navigator.language || navigator.language.toLowerCase().startsWith('zh')

/** 当前页面若是授权弹窗的落脚页，返回要显示的话；否则返回 null。 */
export function embedOAuthLandingMessage(hash: string = typeof location === 'undefined' ? '' : location.hash): string | null {
  const params = new URLSearchParams(hash.replace(/^#/, ''))
  if (params.get('mcp_oauth_result') === 'success') {
    return zh() ? '授权成功，可以关闭此窗口' : 'Authorized. You can close this window.'
  }
  if (params.has('mcp_oauth_error')) {
    return zh() ? '授权没有完成，请关闭此窗口后重试' : 'Authorization was not completed. Close this window and try again.'
  }
  return null
}

/** 落脚页：显示提示并尝试自动关窗。返回 true 表示已接管，调用方不必再往下走。 */
export function handleEmbedOAuthLanding(show: (message: string) => void): boolean {
  const message = embedOAuthLandingMessage()
  if (!message) return false
  show(message)
  // 只有脚本打开的窗口才关得掉；关不掉就留着那句提示，不算错。
  if (message.startsWith('授权成功') || message.startsWith('Authorized')) {
    setTimeout(() => { try { window.close() } catch { /* ignore */ } }, 1500)
  }
  return true
}
