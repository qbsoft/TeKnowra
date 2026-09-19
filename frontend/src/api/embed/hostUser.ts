// 嵌入页「按宿主用户分开存」—— TeKnowra fork 自己的文件，上游没有。
// 设计见 docs/embed-per-user-authorization.md。
//
// 问题：嵌入页在浏览器里按「渠道」存两样东西——匿名访客编号（MCP 按人授权的令牌就挂在它名下）
// 和当前对话的指针。整个浏览器每个渠道只有一份，于是同一台电脑上经理授权过之后，销售员登录
// 宿主系统打开浮窗，用的还是经理的授权、看到的还是经理的对话。
//
// 修法：宿主页面在 WeKnora.init({ hostUser }) 里告诉小部件当前登录的是谁，小部件随令牌一起
// 交给嵌入页；嵌入页把这个标识拼进上面两样东西的存储键。换人 → 换一套；换回来 → 原来那套还在。
// 没传 hostUser 的渠道后缀为空，存储键与上游逐字节一致，行为不变。
//
// 这只是「正常使用下不串号」，不是安全边界：标识由宿主页面自报，访客编号仍是上游那种
// 存在浏览器里、服务端不另行验证的随机数。后端一行不改。
//
// 上游文件里的钩子（共 4 处，各一行）：
//   api/embed/index.ts   embedVisitorStorageKey / embedChatSessionStorageKey 末尾拼 embedHostUserSuffix()
//   api/embed/index.ts   onEmbedHostToken 里调 setEmbedHostUser(e.data.host_user)
//   public/weknora-widget.js  provide_token 消息带上 host_user
// 合并上游后若被冲掉，hostUser.test.ts 会红。

const MAX_LEN = 128

let hostUser = ''

/** 记下宿主页面报上来的当前用户；非字符串/数字、空值一律当作没传。 */
export function setEmbedHostUser(value: unknown): void {
  if (typeof value !== 'string' && typeof value !== 'number') {
    hostUser = ''
    return
  }
  hostUser = String(value).trim().slice(0, MAX_LEN)
}

/** 拼在存储键末尾的后缀；没有宿主用户时为空串（= 上游原样）。 */
export function embedHostUserSuffix(): string {
  return hostUser ? `:u:${encodeURIComponent(hostUser)}` : ''
}
