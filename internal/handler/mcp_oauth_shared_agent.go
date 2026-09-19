package handler

import (
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// 共享智能体场景下的 MCP OAuth —— TeKnowra fork 自己的文件，上游没有。
//
// 问题：A 空间把挂了按人授权 MCP 服务的智能体共享给 B 空间的用户。对话运行时平台用的是
// 源空间（A）去解析模型/知识库/MCP（见 session/qa.go 的 effectiveTenantID），OAuth 令牌
// 也按「源空间 + 本人」去查；但授权这几个接口（取授权地址 / 查状态 / 确认 / 取消）按的是
// 请求方自己的空间（B）——B 空间里根本没有这个 MCP 服务，于是对话里弹出的授权卡片点了
// 就是 "MCP service not found"，确认和查状态同样对不上。结果是共享出去的按人授权智能体
// 别人永远用不了。
//
// 修法：这几个接口接受可选的 ?agent_id=&agent_source_tenant_id=，带了就把空间换成源空间。
// 不带参数时行为与上游完全一致。
//
// 安全边界——这里放行的是「对别的空间的服务发起授权」，所以必须同时满足：
//  1. 请求方空间确实被共享了这个智能体（复用平台自己的 GetSharedAgentForTenant，
//     它同时校验组织成员关系、共享是否仍有效、调用者角色）；
//  2. 这个智能体确实使用了这个 MCP 服务（否则拿一个共享智能体当钥匙，就能对源空间里
//     任意服务发起授权）。
// 任何一条不满足都按无权处理，绝不回退到源空间。令牌仍然记在请求方本人名下，
// 别人的令牌既读不到也写不了。
//
// 上游文件 mcp_oauth.go 里每个接口只改一行（取空间那一行）。合并上游后若被冲掉，
// mcp_oauth_shared_agent_test.go 会红。

const (
	queryAgentID             = "agent_id"
	queryAgentSourceTenantID = "agent_source_tenant_id"
)

// oauthTenantFor 返回本次 OAuth 操作应当使用的空间。
// serviceID 为空表示调用点此刻还不知道服务（如取消），此时只做第 1 条校验；
// 这类调用点随后只会去解决一条属于本人的等待记录，不会触达任何服务。
func (h *MCPOAuthHandler) oauthTenantFor(c *gin.Context, serviceID string) (uint64, error) {
	callerTenant := c.GetUint64(types.TenantIDContextKey.String())
	agentID := strings.TrimSpace(c.Query(queryAgentID))
	rawSource := strings.TrimSpace(c.Query(queryAgentSourceTenantID))
	if agentID == "" || rawSource == "" {
		return callerTenant, nil
	}
	sourceTenant, err := strconv.ParseUint(rawSource, 10, 64)
	if err != nil || sourceTenant == 0 {
		return 0, errors.NewValidationError("invalid " + queryAgentSourceTenantID)
	}
	if sourceTenant == callerTenant {
		return callerTenant, nil
	}
	if callerTenant == 0 || h.agentShare == nil {
		return 0, errors.NewForbiddenError("shared agent access is not available")
	}

	ctx := c.Request.Context()
	agent, err := h.agentShare.GetSharedAgentForTenant(
		ctx, callerTenant, types.TenantRoleFromContext(ctx), agentID, sourceTenant,
	)
	if err != nil || agent == nil || agent.TenantID != sourceTenant {
		return 0, errors.NewForbiddenError("this agent is not shared with your workspace")
	}
	if serviceID != "" && !agentUsesMCPService(agent, serviceID) {
		return 0, errors.NewForbiddenError("this agent does not use the requested MCP service")
	}
	return sourceTenant, nil
}

// agentUsesMCPService 判断智能体配置是否包含该 MCP 服务。
// "all" 表示源空间的全部服务；"selected" 看清单；其余（含空、"none"）一律不算。
func agentUsesMCPService(agent *types.CustomAgent, serviceID string) bool {
	if agent == nil || serviceID == "" {
		return false
	}
	switch agent.Config.MCPSelectionMode {
	case "all":
		return true
	case "selected":
		for _, id := range agent.Config.MCPServices {
			if id == serviceID {
				return true
			}
		}
	}
	return false
}
