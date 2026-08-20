package chat

import (
	"bytes"
	"net/http"
	"testing"
)

// 关于「端到端证明重试真的发生」这件事，这里没有测试，是刻意的。
//
// 试过一版：预热连接池 → srv.CloseClientConnections() → 再发一次。它过了，
// 但把 markReplayable 去掉之后**照样过**——说明它什么都没测到。原因是
// net/http 在复用空闲连接前会探测其存活，发现已关闭就直接重拨，根本走不到
// 「EOF → 判断能否重试」那条分支。真实故障里的竞态是「客户端选中连接的同一
// 瞬间服务端才关」，没法稳定构造。
//
// 所以这里只钉住能确定验证的必要条件：头名字对、GetBody 可用、key 不复用。
// 三者齐备时，net/http 的重试行为由标准库保证（Request.isReplayable）。
// 一个两种情况下都通过的测试，比没有测试更糟——它会让人以为这条路已经验过。

// The header has to actually reach the wire — net/http reads it off the
// request to decide whether a retry is allowed, so a typo in the name would
// silently disable the whole fix.
func TestMarkReplayable_SetsHeaderNetHTTPLooksFor(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://example.invalid", bytes.NewBuffer([]byte(`{}`)))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	markReplayable(req)

	// One of the two names net/http accepts; see Request.isReplayable.
	if req.Header.Get("X-Idempotency-Key") == "" && req.Header.Get("Idempotency-Key") == "" {
		t.Fatal("neither idempotency header was set — net/http will refuse to retry this POST")
	}

	// GetBody is what makes the replay possible at all; bytes.Buffer bodies
	// get it for free, but a future change to a plain io.Reader would not.
	if req.GetBody == nil {
		t.Error("GetBody is nil, so the body cannot be replayed even with the header")
	}
}

// Two requests must not share an idempotency key: a provider that honours the
// header would treat the second as a duplicate of the first and return the
// first response.
func TestMarkReplayable_KeyIsPerRequest(t *testing.T) {
	keys := map[string]bool{}
	for i := 0; i < 5; i++ {
		req, err := http.NewRequest(http.MethodPost, "http://example.invalid", bytes.NewBuffer([]byte(`{}`)))
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		markReplayable(req)
		k := req.Header.Get("X-Idempotency-Key")
		if keys[k] {
			t.Fatalf("idempotency key %q reused across requests", k)
		}
		keys[k] = true
	}
}
