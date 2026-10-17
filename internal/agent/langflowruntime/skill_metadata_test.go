package langflowruntime

import (
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"strings"
	"testing"
)

func TestSkillMetadataAddsDiscoveryWithoutBodyOrChangingInstructionPriority(t *testing.T) {
	live := []chat.Message{{Role: "system", Content: "current instruction"}, {Role: "user", Content: "request"}}
	got := withSkillMetadata(live, []hostcapability.SkillDescriptor{{Name: "tyer-demo", Description: "description"}}, []string{"tyer-demo"})
	if len(got) != 2 || !strings.HasPrefix(got[0].Content, "current instruction") || !strings.Contains(got[0].Content, "tyer-demo") || !strings.Contains(got[0].Content, "按需读取") {
		t.Fatal("skill discovery missing or prompt priority lost")
	}
	if live[0].Content != "current instruction" {
		t.Fatal("live messages aliased")
	}
}

func TestSkillMetadataIgnoresUnboundPinnedNames(t *testing.T) {
	got := withSkillMetadata(nil, []hostcapability.SkillDescriptor{{Name: "tyer-demo"}}, []string{"foreign", "tyer-demo"})
	if strings.Contains(got[0].Content, "foreign") {
		t.Fatal("unbound pinned skill advertised")
	}
}
