package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLangflowFreshInstallOpensReleaseStoreWithoutLegacyBindings(t *testing.T) {
	t.Setenv("TEKNOWRA_LANGFLOW_BINDINGS_FILE", "")
	t.Setenv("TEKNOWRA_LANGFLOW_RELEASES_FILE", filepath.Join(t.TempDir(), "releases.json"))
	c := &Config{}
	if err := c.loadLangflowFile(); err != nil {
		t.Fatal(err)
	}
	if c.LangflowReleases() == nil {
		t.Fatal("fresh installation has no release store")
	}
	if len(c.LangflowDrafts()) != 0 {
		t.Fatal("fresh installation invented draft bindings")
	}
}

func TestLangflowConfiguredInvalidLegacyBindingsStillRejectStartup(t *testing.T) {
	root := t.TempDir()
	bindings := filepath.Join(root, "bindings.json")
	if err := os.WriteFile(bindings, []byte(`{"bindings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEKNOWRA_LANGFLOW_BINDINGS_FILE", bindings)
	t.Setenv("TEKNOWRA_LANGFLOW_RELEASES_FILE", filepath.Join(root, "releases.json"))
	if err := (&Config{}).loadLangflowFile(); err == nil {
		t.Fatal("invalid legacy bindings accepted")
	}
}

func TestLangflowBindingIsolation(t *testing.T) {
	c := &Config{}
	raw := []byte(`{"bindings":[{"agent_id":"lf","flow_id":"flow","workspace_id":1,"models":["m"],"nodes":[{"id":"model","kind":"model"}]}]}`)
	if err := c.loadLangflowBindings(raw); err != nil {
		t.Fatal(err)
	}
	b, ok := c.LangflowBinding("lf")
	if !ok {
		t.Fatal("missing binding")
	}
	b.Models[0] = "foreign"
	original, _ := c.LangflowBinding("lf")
	if original.Models[0] != "m" {
		t.Fatal("caller changed server binding")
	}
	c.einoDefinitions = map[string]string{"lf": "knowledge-summary-v1"}
	if err := c.loadLangflowBindings(raw); err == nil {
		t.Fatal("dual executor binding accepted")
	}
}
