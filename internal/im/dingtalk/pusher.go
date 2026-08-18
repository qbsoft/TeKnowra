package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Tencent/WeKnora/internal/im"
)

// Compile-time proof that DingTalk can be a delivery target for scheduled jobs.
var _ im.Pusher = (*Adapter)(nil)

// Push implements im.Pusher, letting scheduled jobs deliver into DingTalk.
//
// This is deliberately a separate file from adapter.go rather than a refactor
// of replyViaOpenAPI: adapter.go is upstream code, and keeping this out of it
// means the next upstream merge has nothing to resolve here. The overlap is a
// dozen lines of request assembly, which is a cheaper price than a recurring
// conflict.
//
// The one behavioural difference from a reply: there is no IncomingMessage to
// infer routing from, so the target must already say whether this is a group
// or a direct message. It does, because it was captured at job creation time.
func (a *Adapter) Push(ctx context.Context, target im.PushTarget, content string) error {
	if content == "" {
		return nil
	}

	token, err := a.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	msgParam, err := json.Marshal(map[string]string{"title": "定时任务", "text": content})
	if err != nil {
		return fmt.Errorf("marshal msgParam: %w", err)
	}

	body := map[string]interface{}{
		"robotCode": a.clientID,
		"msgKey":    "sampleMarkdown",
		"msgParam":  string(msgParam),
	}

	// A group target keeps the conversation id; otherwise fall back to a
	// direct message to the creator. Anything else is unreachable by
	// construction — the target only ever holds what we captured at creation.
	var apiURL string
	if target.ChatID != "" {
		apiURL = "https://api.dingtalk.com/v1.0/robot/groupMessages/send"
		body["openConversationId"] = target.ChatID
	} else if target.UserID != "" {
		apiURL = "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend"
		body["userIds"] = []string{target.UserID}
	} else {
		return fmt.Errorf("push target has neither chat nor user")
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("push message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("push message: status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
