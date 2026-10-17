package openaicompletions

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/models/api"
	"github.com/stretchr/testify/require"
)

func TestPerCallThinkingControlDoesNotChangeStoredModel(t *testing.T) {
	c := newClient(t, func(cfg *Config) {
		cfg.Settings.ThinkingFormat = api.ThinkingFormatNone
	})
	thinking := false
	messages := []api.Message{{Role: "user", Content: "synthetic"}}
	withOverride := bodyJSON(t, c, messages, &api.Options{Thinking: &thinking, ThinkingControl: "thinking_type"}, true)
	require.Equal(t, map[string]any{"type": "disabled"}, withOverride["thinking"])
	withoutOverride := bodyJSON(t, c, messages, &api.Options{Thinking: &thinking}, true)
	require.NotContains(t, withoutOverride, "thinking")
	require.Equal(t, api.ThinkingFormatNone, c.cfg.Settings.ThinkingFormat)
}
