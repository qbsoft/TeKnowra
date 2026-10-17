package einoruntime

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/einodef"
	"github.com/Tencent/WeKnora/internal/types"
)

//go:embed *.go
var implementationSource embed.FS

// A fingerprint is baked into the executable, never read from mutable disk.
// Test files are intentionally excluded; catalog and implementation both count.
func ImplementationVersion(definition string) string {
	definitionManifest, ok := einodef.Lookup(definition)
	if !ok {
		return ""
	}
	h := sha256.New()
	_ = fs.WalkDir(implementationSource, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, e := implementationSource.ReadFile(path)
		if e != nil {
			return e
		}
		h.Write([]byte(path + "\x00"))
		h.Write([]byte(strings.ReplaceAll(string(raw), "\r\n", "\n")))
		h.Write([]byte{0})
		return nil
	})
	raw, _ := json.Marshal(definitionManifest)
	h.Write(raw)
	return "eino-sha256:" + hex.EncodeToString(h.Sum(nil))
}

func (r *run) recordAudit(raw string) {
	node := "validate"
	if len(r.state.RoundSteps) > 0 {
		node = r.state.RoundSteps[len(r.state.RoundSteps)-1].NodeKey
	}
	if d := types.AuditDiagnostic("eino", node, raw); d != nil {
		d.Definition = r.engine.opts.Definition
		d.Implementation = ImplementationVersion(d.Definition)
		d.RunID = r.runID
		r.state.ExecutionDiagnostics = append(r.state.ExecutionDiagnostics, *d)
	}
}
