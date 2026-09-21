package session

import (
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// 「来源空间就是自己的空间」不算共享 —— TeKnowra fork 自己的文件，上游没有。
//
// 问题：请求里的 agent_source_tenant_id 表示「这是别的空间共享给我的智能体，来源空间是 X」。
// 上游 resolveAgent 只要 X 不为 0 就只按共享关系去查，查不到就 404 "Shared agent not found"，
// 且不回退到自己的智能体（防止悄悄跑成一个同 ID 的本地内置智能体——这个顾虑是对的）。
// 但 X 等于调用者自己的空间时，这条规则把人锁在了门外：空间所有者把自己的智能体共享进了
// 共享空间，又从「共享空间」页面点「在对话中使用」，前端记下的来源空间就是他自己的空间；
// 这个选择存在浏览器里，此后每次提问都是 404「流式连接失败」，换对话也没用。
// 2026-09-21 线上 admin 撞上（本机浏览器里存的来源空间是空的，所以本机测不出来）。
//
// 修法：来源空间等于调用者当前空间时，视为没带来源空间——那就是他自己的智能体，
// 走上游「自己的智能体」那条路。来源是别的空间时一字不改，上游的防线原样保留。
//
// 钩子：qa.go 的 resolveAgent 开头一行。被上游冲掉的话 agent_own_source_teknowra_test.go 会红。
func ownTenantIsNotAShareSource(c *gin.Context, sourceTenantID uint64) uint64 {
	if sourceTenantID == 0 || c == nil {
		return sourceTenantID
	}
	if current := c.GetUint64(types.TenantIDContextKey.String()); current != 0 && current == sourceTenantID {
		return 0
	}
	return sourceTenantID
}
