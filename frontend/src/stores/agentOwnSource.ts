// 「来源空间就是自己的空间」不算共享 —— TeKnowra fork 自己的文件，上游没有。
//
// selectedAgentSourceTenantId 表示「当前选的是别的空间共享给我的智能体，来源空间是 X」，前端会把它带进
// 对话、知识库查询、附件上传、MCP 授权等一串请求。空间所有者从「共享空间」页面点开自己共享出去的智能体
// 并「在对话中使用」时，X 就是他自己的空间；后端按共享关系去查，自己没有共享给自己 → 404
// 「流式连接失败」。这个选择持久化在浏览器里，换对话也没用。（2026-09-21 线上 admin 撞上。）
//
// 后端在对话入口也做了同样的归一（handler/session/agent_own_source_teknowra.go）；这里从源头不发出
// 这个值，其余带这个参数的接口也就一并不受影响，而且读取时归一能把浏览器里已经存下的旧值也救回来。
//
// 钩子：stores/settings.ts 的 getter 和 selectAgent 各一处。被上游冲掉的话 agentOwnSource.test.ts 会红。

/** 来源空间等于当前空间（或为空）时返回 null，否则原样返回字符串。 */
export function normalizeAgentSourceTenant(
  sourceTenantId: string | number | null | undefined,
  currentTenantId: string | number | null | undefined,
): string | null {
  if (sourceTenantId == null) return null
  const source = String(sourceTenantId).trim()
  if (!source) return null
  const current = currentTenantId == null ? '' : String(currentTenantId).trim()
  return current && current === source ? null : source
}

/** 当前空间号。直接读 auth store 持久化的那一份，避免 settings ↔ auth 两个 store 互相引用。 */
export function currentTenantIdFromStorage(): string {
  try {
    const raw = localStorage.getItem('weknora_tenant')
    if (!raw) return ''
    const id = JSON.parse(raw)?.id
    return id == null ? '' : String(id)
  } catch {
    return ''
  }
}
