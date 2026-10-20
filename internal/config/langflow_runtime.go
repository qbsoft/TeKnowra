package config

import (
	"fmt"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"os"
)

// LangflowBinding is service-controlled; request payloads cannot choose an engine.
func (c *Config) LangflowBinding(id string) (langflowdef.Binding, bool) {
	if c == nil {
		return langflowdef.Binding{}, false
	}
	b, ok := c.DraftLangflowBinding(id)
	if ok && c.langflowReleases != nil {
		if r, active := c.langflowReleases.Active(id); active {
			if r.WorkspaceID != b.WorkspaceID {
				return langflowdef.Binding{}, false
			}
			b = r.Binding.Clone()
			b.ReleaseID = r.ID
			b.GraphDigest = r.GraphDigest
			b.RuntimeOwnerID = r.RuntimeOwnerID
		}
	}
	return b.Clone(), ok
}
func (c *Config) DraftLangflowBinding(id string) (langflowdef.Binding, bool) {
	if c == nil {
		return langflowdef.Binding{}, false
	}
	b, ok := c.langflowBindings[id]
	if c.langflowReleases != nil {
		if managed, found := c.langflowReleases.Draft(id); found {
			return managed, true
		}
	}
	return b.Clone(), ok
}
func (c *Config) LangflowReleases() *langflowrelease.Store {
	if c == nil {
		return nil
	}
	return c.langflowReleases
}

func (c *Config) LangflowDrafts() []langflowdef.Binding {
	result := []langflowdef.Binding{}
	seen := map[string]bool{}
	if c == nil {
		return result
	}
	if c.langflowReleases != nil {
		for _, b := range c.langflowReleases.Drafts() {
			result = append(result, b)
			seen[b.AgentID] = true
		}
	}
	for _, b := range c.langflowBindings {
		if !seen[b.AgentID] {
			result = append(result, b.Clone())
		}
	}
	return result
}
func (c *Config) loadLangflowBindings(raw []byte) error {
	bindings, err := langflowdef.ParseBindings(raw)
	if err != nil {
		return err
	}
	for id := range bindings {
		if _, ok := c.EinoDefinition(id); ok {
			return fmt.Errorf("Agent cannot bind both Eino and Langflow")
		}
	}
	c.langflowBindings = bindings
	return nil
}
func (c *Config) loadLangflowFile() error {
	path := os.Getenv("TEKNOWRA_LANGFLOW_BINDINGS_FILE")
	if path != "" {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1024*1024 {
			return fmt.Errorf("Langflow bindings file unavailable or too large")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Langflow bindings file unavailable")
		}
		if err = c.loadLangflowBindings(raw); err != nil {
			return err
		}
	}
	if releasePath := os.Getenv("TEKNOWRA_LANGFLOW_RELEASES_FILE"); releasePath != "" {
		var err error
		c.langflowReleases, err = langflowrelease.Open(releasePath)
		if err != nil {
			return err
		}
	}
	return nil
}
