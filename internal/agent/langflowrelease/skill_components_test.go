package langflowrelease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillComponentProfilePinsSkillCodeWithoutChangingLegacyProfile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "components/teknowra/skills.py")
	os.MkdirAll(filepath.Dir(file), 0700)
	os.WriteFile(file, []byte("skill code"), 0600)
	base := Components{Digest: strings.Repeat("a", 64), Approved: map[string]string{"Agent": "native"}}
	next, err := WithSkillComponent(base, root)
	if err != nil {
		t.Fatal(err)
	}
	if base.Approved["TeKnowraSkills"] != "" || next.Approved["TeKnowraSkills"] != "skill code" || next.Digest == base.Digest {
		t.Fatal("profile isolation lost")
	}
	os.WriteFile(file, []byte("updated"), 0600)
	changed, err := WithSkillComponent(base, root)
	if err != nil || changed.Digest == next.Digest {
		t.Fatal("skill source mutation not pinned")
	}
}
