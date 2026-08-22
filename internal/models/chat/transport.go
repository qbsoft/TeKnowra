package chat

import (
	"context"
	"github.com/google/uuid"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	secutils "github.com/Tencent/WeKnora/internal/utils"
)

// LLM 调用超时配置。仅作为"上层未设置 deadline 时"的兜底，避免 hung 请求
// 永久阻塞 worker。如果上层 ctx 已经设置了 deadline（无论比默认更短还是更长），
// 都会原样尊重，不再叠加默认超时。可通过环境变量覆盖：
//   - WEKNORA_LLM_CHAT_TIMEOUT_SECONDS    非流式调用兜底超时（默认 600s）
//   - WEKNORA_LLM_STREAM_TIMEOUT_SECONDS  流式调用兜底超时（默认 1800s）
var (
	defaultChatTimeout   = envDurationSeconds("WEKNORA_LLM_CHAT_TIMEOUT_SECONDS", 300*time.Second)
	defaultStreamTimeout = envDurationSeconds("WEKNORA_LLM_STREAM_TIMEOUT_SECONDS", 600*time.Second)
)

// envDurationSeconds 读取以"秒"为单位的环境变量，解析失败或非正值时回退到 fallback。
func envDurationSeconds(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return time.Duration(n) * time.Second
}

// withLLMTimeout 仅在上层 ctx 没有 deadline 时附加一个兜底超时；
// 如果上层已显式设置 deadline（无论更短或更长），则原样返回，
// 让调用方对自己的超时策略拥有最终决定权。
func withLLMTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

// rawHTTPClient is a shared HTTP client for raw HTTP LLM calls with connection-level timeouts.
// Per-request timeout is enforced via context deadline (see defaultChatTimeout / defaultStreamTimeout)
// rather than http.Client.Timeout, so streaming calls are not prematurely terminated.
// Uses SSRFSafeDialContext to prevent DNS rebinding attacks at the connection layer.
var rawHTTPTransport = &http.Transport{
	Proxy:               http.ProxyFromEnvironment,
	DialContext:         secutils.SSRFSafeDialContext,
	TLSHandshakeTimeout: 10 * time.Second,
	// Shorter than the 90s default on purpose. Providers and the gateways in
	// front of them commonly drop idle connections around 60s without telling
	// the client; reusing one of those fails immediately with EOF. Pruning
	// ours first turns "first request after a pause always fails" into a fresh
	// dial nobody notices.
	IdleConnTimeout:     30 * time.Second,
	MaxIdleConnsPerHost: 5,
}

var rawHTTPClient = secutils.NewSSRFSafeHTTPClientWithTransport(
	secutils.SSRFSafeHTTPClientConfig{Timeout: 0, MaxRedirects: 10},
	rawHTTPTransport,
)

// markReplayable tells net/http that this POST may be retried on a connection
// that turned out to be dead.
//
// The failure it fixes: an idle pooled connection the provider already closed
// is picked for the next request, which fails with a bare EOF. net/http is
// willing to retry that on a reused connection — but first it asks
// Request.isReplayable(), and a POST only qualifies if it carries an
// idempotency header:
//
//	// net/http/request.go
//	if r.Header.has("Idempotency-Key") || r.Header.has("X-Idempotency-Key") {
//	    return true
//	}
//
// Without the header the retry is refused and the EOF surfaces to the user,
// which is why the first message after a pause failed and the second always
// worked — the second got a fresh connection.
//
// Safe here because the retry only fires when nothing was written or the
// server closed an idle connection, i.e. when the provider never saw the
// request. The body is a *bytes.Buffer, so GetBody is already set and the
// request can genuinely be replayed.
func markReplayable(req *http.Request) {
	req.Header.Set("X-Idempotency-Key", uuid.NewString())
}
