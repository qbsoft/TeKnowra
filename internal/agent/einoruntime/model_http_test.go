package einoruntime

import (
	"context"
	"fmt"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNativeHTTPTruncatedStreamCannotCompleteReview(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	utils.ResetSSRFWhitelistForTest()
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"{}\"}}]}\n\n")
	}))
	defer server.Close()
	model, err := chat.NewChat(&chat.ChatConfig{Source: types.ModelSourceRemote, BaseURL: server.URL + "/v1", ModelName: "synthetic", Provider: "openai", APIKey: "synthetic-not-secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReadModel(context.Background(), model, []chat.Message{{Role: "user", Content: "synthetic"}}, nil); err == nil {
		t.Fatal("native HTTP EOF without provider finish accepted")
	}
}

func TestWrappedNativeHTTPPreservesFinishAndTrailingUsage(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	utils.ResetSSRFWhitelistForTest()
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	for _, reportUsage := range []bool{false, true} {
		t.Run(fmt.Sprint(reportUsage), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"{}\"}}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
				if reportUsage {
					fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":2,\"total_tokens\":9}}\n\n")
				}
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer server.Close()
			model, err := chat.NewChat(&chat.ChatConfig{Source: types.ModelSourceRemote, BaseURL: server.URL + "/v1", ModelName: "synthetic", Provider: "openai", APIKey: "synthetic-not-secret"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			r, err := ReadModel(context.Background(), model, []chat.Message{{Role: "user", Content: "synthetic"}}, nil)
			if err != nil || r.Content != "{}" || r.FinishReason != "stop" || r.UsageReported != reportUsage {
				t.Fatalf("r=%+v err=%v", r, err)
			}
			if reportUsage && r.Usage.TotalTokens != 9 {
				t.Fatal("trailing usage lost")
			}
		})
	}
}
