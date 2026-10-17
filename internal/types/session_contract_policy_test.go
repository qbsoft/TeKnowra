package types

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSessionContractPolicyPersistsInExistingJSON(t *testing.T) {
	raw, err := json.Marshal(MessageExecutionContext{ChatHistoryIndexingDisabled: true})
	require.NoError(t, err)
	var restored MessageExecutionContext
	require.NoError(t, json.Unmarshal(raw, &restored))
	require.True(t, restored.ChatHistoryIndexingDisabled)
	var legacy MessageExecutionContext
	require.NoError(t, json.Unmarshal([]byte(`{}`), &legacy))
	require.False(t, legacy.ChatHistoryIndexingDisabled)
	raw, err = json.Marshal(CustomAgentConfig{DisableChatHistoryIndexing: true, AllowSessionAttachments: true})
	require.NoError(t, err)
	var cfg CustomAgentConfig
	require.NoError(t, json.Unmarshal(raw, &cfg))
	require.True(t, cfg.DisableChatHistoryIndexing)
	require.True(t, cfg.AllowSessionAttachments)
}
