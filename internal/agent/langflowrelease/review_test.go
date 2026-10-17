package langflowrelease

import (
	"context"
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/langflowapi"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewGraphRejectsUnboundAgentAndFilesBeforeImport(t *testing.T) {
	b := testRelease("v1").Binding
	b.FlowID = "draft"
	for _, extra := range []string{
		`{"id":"agent","data":{"type":"Agent","node":{"template":{"code":{"value":"native"},"model":{"value":[{"provider":"OpenAI"}]}}}}}`,
		`{"id":"input","data":{"type":"ChatInput","node":{"template":{"code":{"value":"chat"},"files":{"value":["mutable.pdf"]}}}}}`,
	} {
		data := json.RawMessage(`{"nodes":[{"id":"model","data":{"type":"TeKnowraModel","node":{"template":{"code":{"value":"approved"},"model_id":{"value":"m"}}}}},` + extra + `],"edges":[]}`)
		s, _ := Open(filepath.Join(t.TempDir(), "store.json"))
		editor := &fakeFlows{flow: langflowapi.Flow{ID: "draft", UserID: "editor", Data: data}}
		runtime := &fakeFlows{flow: langflowapi.Flow{ID: "runtime", UserID: "runner"}}
		m := Manager{Store: s, Editor: editor, Runtime: runtime, RuntimeOwnerID: "runner"}
		_, err := m.Publish(context.Background(), b, "V1", Components{Digest: strings.Repeat("a", 64), Approved: map[string]string{"TeKnowraModel": "approved", "Agent": "native", "ChatInput": "chat"}}, strings.Repeat("b", 64))
		if err == nil || runtime.flow.Data != nil || len(s.List("a")) != 0 {
			t.Error("external dependency imported or activated")
		}
	}
}

func TestReviewOversizedAppendKeepsReopenableStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	s, _ := Open(path)
	if err := s.Add(testRelease("v1")); err != nil {
		t.Fatal(err)
	}
	if err := s.Activate("a", "v1"); err != nil {
		t.Fatal(err)
	}
	huge := testRelease("huge")
	huge.Label = strings.Repeat("x", 16*1024*1024)
	if s.Add(huge) == nil {
		t.Fatal("oversized store accepted")
	}
	active, _ := s.Active("a")
	if active.ID != "v1" {
		t.Fatal("active changed")
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reopened.Get("a", "huge"); ok {
		t.Fatal("oversized release persisted")
	}
}

func TestReviewAgentRequiresExactlyOneHostModelEdge(t *testing.T) {
	b := testRelease("v1").Binding
	nodes := `"nodes":[{"id":"model","data":{"type":"ext:teknowra:TeKnowraModel@installed","node":{"template":{"code":{"value":"approved"},"model_id":{"value":"m"}}}}},{"id":"agent","data":{"type":"Agent","node":{"template":{"code":{"value":"native"},"model":{"value":[{"provider":"ignored UI default"}]}}}}}]`
	edge := `{"source":"model","target":"agent","targetHandle":"{\"fieldName\":\"model\",\"id\":\"agent\"}"}`
	approved := map[string]string{"TeKnowraModel": "approved", "Agent": "native"}
	if err := ValidateGraph(json.RawMessage(`{`+nodes+`,"edges":[`+edge+`]}`), b, approved); err != nil {
		t.Fatal(err)
	}
	for _, edges := range []string{"", edge + "," + edge, strings.ReplaceAll(edge, `\"model\"`, `\"tools\"`)} {
		if ValidateGraph(json.RawMessage(`{`+nodes+`,"edges":[`+edges+`]}`), b, approved) == nil {
			t.Fatal("unbound or duplicate model connection accepted")
		}
	}
}

func TestReviewDirectAgentDependenciesChangeFingerprint(t *testing.T) {
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
	for _, n := range []string{"components/teknowra/model.py", "components/teknowra/knowledge.py", "components/teknowra/mcp.py", "teknowra_support.py"} {
		write(filepath.Join(root, n), "approved")
	}
	for _, n := range []string{"lfx/components/models_and_agents/agent.py", "lfx/components/input_output/chat.py", "lfx/components/input_output/chat_output.py"} {
		write(filepath.Join(lib, n), "native")
	}
	for _, n := range []string{"lfx", "langflow_base", "langchain_core", "langchain_openai", "pydantic", "jsonschema", "httpx", "langchain", "langgraph", "langgraph_checkpoint", "langgraph_prebuilt", "langgraph_sdk"} {
		write(filepath.Join(lib, n+"-1.dist-info/METADATA"), "Version: 1\n")
	}
	original, err := LocalComponents(context.Background(), &fakeVersion{"1.12.5"}, root, lib)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"langchain", "langgraph", "langgraph_checkpoint", "langgraph_prebuilt", "langgraph_sdk"} {
		write(filepath.Join(lib, n+"-1.dist-info/METADATA"), "Version: 2\n")
		changed, err := LocalComponents(context.Background(), &fakeVersion{"1.12.5"}, root, lib)
		if err != nil || changed.Digest == original.Digest {
			t.Errorf("%s upgrade not detected", n)
			continue
		}
		m := Manager{RuntimeOwnerID: "runner"}
		r := testRelease("v1")
		r.ComponentsDigest = original.Digest
		if m.Verify(context.Background(), r, changed.Digest, r.ToolsDigest) == nil {
			t.Fatal("dependency upgrade accepted")
		}
		write(filepath.Join(lib, n+"-1.dist-info/METADATA"), "Version: 1\n")
	}
}
