package langflowrelease

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type fakeVersion struct{ value string }

func (f *fakeVersion) Version(context.Context) (string, error) { return f.value, nil }
func TestComponentsPinSupportAndInstalledDependencies(t *testing.T) {
	root := t.TempDir()
	lib := filepath.Join(root, "lib")
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"components/teknowra/model.py", "components/teknowra/knowledge.py", "components/teknowra/mcp.py", "teknowra_support.py"} {
		write(filepath.Join(root, name), "approved")
	}
	for _, name := range []string{"lfx/components/models_and_agents/agent.py", "lfx/components/input_output/chat.py", "lfx/components/input_output/chat_output.py"} {
		write(filepath.Join(lib, name), "native")
	}
	for _, name := range []string{"lfx", "langflow_base", "langchain_core", "langchain_openai", "langchain", "langgraph", "langgraph_checkpoint", "langgraph_prebuilt", "langgraph_sdk", "pydantic", "jsonschema", "httpx"} {
		write(filepath.Join(lib, name+"-1.dist-info/METADATA"), "Version: 1\n")
	}
	version := &fakeVersion{"1.12.5"}
	first, err := LocalComponents(context.Background(), version, root, lib)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(root, "teknowra_support.py"), "changed support")
	next, err := LocalComponents(context.Background(), version, root, lib)
	if err != nil || first.Digest == next.Digest {
		t.Fatal("support change not detected")
	}
	version.value = "1.12.6"
	upgrade, _ := LocalComponents(context.Background(), version, root, lib)
	if upgrade.Digest == next.Digest {
		t.Fatal("runtime upgrade not detected")
	}
}
