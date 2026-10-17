package service

import (
	"context"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/types"
	"strings"
	"testing"
)

func TestLangflowSkillsRejectOldSessionSandboxAfterAgentMoves(t *testing.T) {
	ctx := context.Background()
	db := newPinTestDB(t)
	pinner := NewSessionSandboxPinner(db)
	binding := langflowdef.Binding{WorkspaceID: 7, SandboxConfigID: "cfg-b"}
	if err := validateLangflowSkillSession(ctx, pinner, binding, "s-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := pinner.Pin(ctx, "s-1", SandboxPin{ConfigID: "cfg-a", TenantID: 7}); err != nil {
		t.Fatal(err)
	}
	if validateLangflowSkillSession(ctx, pinner, binding, "s-1") == nil {
		t.Fatal("old A sandbox accepted for B skill binding")
	}
	binding.SandboxConfigID = "cfg-a"
	if err := validateLangflowSkillSession(ctx, pinner, binding, "s-1"); err != nil {
		t.Fatal(err)
	}
	binding.WorkspaceID = 8
	if validateLangflowSkillSession(ctx, pinner, binding, "s-1") == nil {
		t.Fatal("foreign workspace pin accepted")
	}
	binding.WorkspaceID = 7
	sqlDB, _ := db.DB()
	sqlDB.Close()
	if validateLangflowSkillSession(ctx, pinner, binding, "s-1") == nil {
		t.Fatal("pin read failure silently allowed")
	}
	if validateLangflowSkillSession(ctx, nil, binding, "s-1") == nil {
		t.Fatal("missing pin store silently allowed")
	}
}

func TestLangflowSkillsRespectAgentSelectionAndInstalledVersion(t *testing.T) {
	b := langflowdef.Binding{WorkspaceID: 1, Skills: []string{"tyer-demo"}, SandboxConfigID: "sandbox"}
	c := types.CustomAgentConfig{SkillsSelectionMode: "selected", SelectedSkills: []string{"tyer-demo"}, SandboxConfigID: "sandbox"}
	if err := validateLangflowSkillGrant(b, c); err != nil {
		t.Fatal(err)
	}
	c.SelectedSkills = []string{"foreign"}
	if validateLangflowSkillGrant(b, c) == nil {
		t.Fatal("unselected skill granted")
	}
	c.SelectedSkills = []string{"tyer-demo"}
	c.SandboxConfigID = "other"
	if validateLangflowSkillGrant(b, c) == nil {
		t.Fatal("another sandbox granted")
	}
	row := &types.TenantSkillEntity{Name: "tyer-demo", TenantID: 1, SandboxConfigID: "sandbox", Enabled: true, Status: types.SkillStatusReady, Instructions: "original", BundleSHA256: strings.Repeat("a", 64)}
	d, err := langflowSkillDigest(b, []*types.TenantSkillEntity{row})
	if err != nil {
		t.Fatal(err)
	}
	row.Instructions = "updated"
	next, err := langflowSkillDigest(b, []*types.TenantSkillEntity{row})
	if err != nil || d == next {
		t.Fatal("skill instruction update not pinned")
	}
	row.Enabled = false
	if _, err = langflowSkillDigest(b, []*types.TenantSkillEntity{row}); err == nil {
		t.Fatal("disabled skill usable")
	}
}
func TestLangflowSkillScriptEntryCannotBecomeArbitraryShell(t *testing.T) {
	command, err := skillScriptCommand("/opt/skills/tyer-demo", "scripts/report.py", []string{"a'b", "$(touch /tmp/escape)", "two words"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(command, "'$(touch /tmp/escape)'") || !strings.Contains(command, "'a'\"'\"'b'") {
		t.Fatal("script arguments lost literal quoting", command)
	}
	for _, entry := range []string{"../foreign/run.py", "/tmp/run.py", "scripts/../../foreign/run.py", "scripts/run.py; id", "scripts/run.exe", "scripts\\run.py"} {
		if _, err := skillScriptCommand("/opt/skills/tyer-demo", entry, nil); err == nil {
			t.Fatal("unsafe script entry accepted", entry)
		}
	}
}

func TestLangflowSkillToolAliasesMatchPythonNames(t *testing.T) {
	for _, name := range []string{"find_customer", strings.Repeat("a", 45), "客户.lookup-v2"} {
		expected := name
		if name == strings.Repeat("a", 45) {
			expected = strings.Repeat("a", 40)
		}
		if name == "客户.lookup-v2" {
			expected = "___lookup-v2"
		}
		got := langflowToolAlias("service", name)
		if !strings.HasSuffix(got, "_"+expected) || len(got) > 56 {
			t.Fatalf("incompatible alias: %s", got)
		}
	}
	if langflowToolAlias("one", "same") == langflowToolAlias("two", "same") {
		t.Fatal("service namespace lost")
	}
}

func TestLangflowSkillReadPaginationReachesNativeReader(t *testing.T) {
	in, err := langflowSkillReadInput(hostcapability.SkillInvocation{Name: "tyer-demo", FilePath: "REFERENCE.md", Offset: 2001, Limit: 500, MaxBytes: 4096})
	if err != nil || in.Offset != 2001 || in.Limit != 500 || in.MaxBytes != 4096 || in.Path != "skill://tyer-demo/REFERENCE.md" {
		t.Fatalf("pagination lost: %+v %v", in, err)
	}
	for _, offset := range []int{-1, 1000001} {
		if _, err := langflowSkillReadInput(hostcapability.SkillInvocation{Name: "tyer-demo", Offset: offset}); err == nil {
			t.Fatal("invalid pagination allowed")
		}
	}
}
