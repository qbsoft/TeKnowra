package service

import (
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/langflowcatalog"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAttachmentSkillCompositionSurvivesReleaseAndRuntimeChecks(t *testing.T) {
	no := false
	b := langflowdef.Binding{AllowSessionAttachments: true, Skills: []string{"review"}, SandboxConfigID: "sandbox"}
	cfg := &types.AgentConfig{MemoryEnabled: &no, AllowSessionAttachments: true, SkillsEnabled: true, AllowedSkills: []string{"review"}, SandboxConfigID: "sandbox"}
	catalog := langflowcatalog.Catalog{AllowSessionAttachments: true, Skills: []langflowcatalog.SkillResource{{ID: "review"}}}
	require.NoError(t, validateLangflowCapabilities(b, cfg))
	require.NoError(t, completeReleaseCatalog(catalog, b))
	b.AllowSkillScripts = true
	require.Error(t, validateLangflowCapabilities(b, cfg))
	require.Error(t, completeReleaseCatalog(catalog, b))
	b.StructuredScripts = []langflowdef.StructuredScript{{Skill: "review", Path: "scripts/calculate.py", InputSchema: json.RawMessage(`{"type":"object","properties":{"amount":{"type":"number","minimum":0}},"required":["amount"],"additionalProperties":false}`)}}
	require.NoError(t, validateLangflowCapabilities(b, cfg))
	require.NoError(t, completeReleaseCatalog(catalog, b))
	cfg.AllowedSkills = []string{"foreign"}
	require.Error(t, validateLangflowCapabilities(b, cfg))
	catalog.Skills = nil
	require.Error(t, completeReleaseCatalog(catalog, b))
}

func TestLangflowAttachmentsRequireMatchingPublication(t *testing.T) {
	no := false
	for _, published := range []bool{false, true} {
		for _, current := range []bool{false, true} {
			b := langflowdef.Binding{AllowSessionAttachments: published}
			cfg := &types.AgentConfig{MemoryEnabled: &no, AllowSessionAttachments: current}
			require.Equal(t, published != current, validateLangflowCapabilities(b, cfg) != nil)
			require.Equal(t, published != current, completeReleaseCatalog(langflowcatalog.Catalog{AllowSessionAttachments: current}, b) != nil)
		}
	}
}
