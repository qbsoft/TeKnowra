package langflowrelease

import (
	"context"
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"os"
	"testing"
)

// Optional local integration fixture: official saved graph plus installed sources.
// The fixture must be scrubbed of tickets/capabilities before it is exported.
func TestLocalOfficialSavedGraph(t *testing.T) {
	path := os.Getenv("TEKNOWRA_RELEASE_GRAPH_FIXTURE")
	if path == "" {
		t.Skip("local saved graph fixture not configured")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Binding langflowdef.Binding `json:"binding"`
		Graph   json.RawMessage     `json:"graph"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	components, err := LocalComponents(context.Background(), &fakeVersion{"local-fixture"}, os.Getenv("TEKNOWRA_RELEASE_COMPONENT_FIXTURE"), os.Getenv("TEKNOWRA_RELEASE_LIBRARY_FIXTURE"))
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateGraph(fixture.Graph, fixture.Binding, components.Approved); err != nil {
		t.Fatal(err)
	}
}
