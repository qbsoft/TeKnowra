package handler

import (
	"context"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// 共享智能体的「开场推荐问题」—— TeKnowra fork 自己的文件，上游没有。
//
// 问题：取推荐问题的接口只在调用者自己的空间里找智能体。别的空间共享过来的智能体（线上 sales01 用的
// 「经营问数」）在他自己空间里没有，于是每次都是 "agent not found"，开场的几个推荐问题永远不显示。
// 对话本身不受影响——对话接口早就会按 agent_source_tenant_id 换到来源空间（session/qa.go resolveAgent）。
//
// 修法：这个接口也接受可选的 ?agent_source_tenant_id=。带了且不是自己的空间时：
//  1. 用平台自己的 GetSharedAgentForTenant 校验「这个智能体确实共享给了你的空间」，不满足就拒绝；
//  2. 通过后把这次请求的空间换成来源空间，服务层照常读智能体配置；
//  3. 丢掉调用者自带的知识库 / 文档 / 标签范围，只按智能体自己配置的范围取——否则可以拿一个共享
//     智能体当钥匙，读来源空间里任意知识库的 FAQ 问题。
//
// 不带参数、或来源就是自己的空间时，行为与上游完全一致。
//
// 钩子：custom_agent.go 的 GetSuggestedQuestions 里一行，被冲掉的话 custom_agent_shared_suggestions_test.go 会红。
func (h *CustomAgentHandler) sharedAgentSuggestionScope(
	c *gin.Context,
	ctx context.Context,
	agentID string,
	kbIDs, knowledgeIDs []string,
	tagScopes []types.TagScope,
) (context.Context, []string, []string, []types.TagScope, error) {
	source, err := types.ParseAgentSourceTenantID(c.Query(types.AgentSourceTenantIDParam))
	if err != nil {
		return ctx, nil, nil, nil, errors.NewBadRequestError("invalid " + types.AgentSourceTenantIDParam)
	}
	caller := c.GetUint64(types.TenantIDContextKey.String())
	if source == 0 || source == caller {
		return ctx, kbIDs, knowledgeIDs, tagScopes, nil
	}
	if caller == 0 || h.agentShare == nil {
		return ctx, nil, nil, nil, errors.NewForbiddenError("shared agent access is not available")
	}
	agent, err := h.agentShare.GetSharedAgentForTenant(ctx, caller, types.TenantRoleFromContext(ctx), agentID, source)
	if err != nil || agent == nil || agent.TenantID != source {
		return ctx, nil, nil, nil, errors.NewNotFoundError("Agent not found")
	}
	return types.WithExecutionTenant(ctx, source), nil, nil, nil, nil
}
