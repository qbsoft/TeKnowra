package mcp

import (
	"context"
	"net/url"
	"os"
	"strings"
)

// 授权回跳地址只认一个 —— TeKnowra fork 自己的文件，上游没有。
//
// 问题：平台对每个 MCP 服务只向对方做一次动态登记，并把当时的回跳地址一起登记过去
// （mcp_oauth_clients.redirect_uri）。而网页发起授权时，回跳地址是前端用
// window.location.origin 拼的——用户这次从哪个地址打开平台，就是哪个。于是同一个平台
// 换个地址进来（域名 / 内网 IP / localhost / 127.0.0.1），授权就被对方拒掉：
// "Redirect URI ... not registered for client"，配了回跳地址白名单的 MCP 服务则报
// "does not match allowed patterns"。用户看到的是一屏 JSON。本机三天内踩了三次，
// 线上只要有人不走域名也一样。
//
// 修法：回跳地址由服务端说了算，不再采信前端报上来的。
//  1. 这个服务已经登记过 → 一律用登记时的那个地址（它是唯一对方认的）。
//  2. 还没登记过 → 配了 APP_EXTERNAL_URL 就用它拼（与 IM 机器人生成授权链接的口径一致，
//     见 im/service.go 的 mcpOAuthCallbackURL），否则才用前端报的。
// 回调是一个不需要登录态的后端接口，凭 state 找回上下文，所以它落在哪个地址上都能完成；
// 完成后再跳回用户原来所在的页面（frontend_redirect）。
//
// 顺带的好处：回跳地址不再由浏览器一端决定。
//
// 钩子：handler/mcp_oauth.go 的 AuthorizeURL 里一行。被上游冲掉的话
// handler/mcp_oauth_canonical_redirect_test.go 会红。

const oauthCallbackPath = "/api/v1/mcp-oauth/callback"

// CanonicalRedirectURI 返回本次授权应当使用的回跳地址。
func (m *OAuthManager) CanonicalRedirectURI(
	ctx context.Context, tenantID uint64, serviceID, clientSupplied string,
) string {
	if m != nil && m.repo != nil && serviceID != "" {
		if existing, err := m.repo.GetClient(ctx, tenantID, serviceID); err == nil && existing != nil {
			if registered := strings.TrimSpace(existing.RedirectURI); registered != "" {
				return registered
			}
		}
	}
	if configured := externalCallbackURL(os.Getenv("APP_EXTERNAL_URL")); configured != "" {
		return configured
	}
	return clientSupplied
}

// externalCallbackURL 由平台对外地址拼出回调地址；地址不像样就返回空（宁可退回前端报的，
// 也不要拿一个坏地址去登记——登记只有一次）。
func externalCallbackURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return ""
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	return base + oauthCallbackPath
}
