//go:build langflow_delivery

package langflowruntime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/langflowapi"
	"github.com/Tencent/WeKnora/internal/agent/langflowdef"
	"github.com/Tencent/WeKnora/internal/agent/langflowrelease"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
)

type deliveryModel struct{ calls int }

func (m *deliveryModel) GetModelName() string { return "delivery-synthetic-model" }
func (m *deliveryModel) GetModelID() string   { return m.GetModelName() }
func (m *deliveryModel) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	panic("stream required")
}
func (m *deliveryModel) ChatStream(context.Context, []chat.Message, *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	m.calls++
	out := make(chan types.StreamResponse, 1)
	out <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: "DELIVERY-HOST-MODEL-OK", Done: true, FinishReason: "stop"}
	close(out)
	return out, nil
}

// Opt-in test against a fresh authenticated local Langflow. It does not touch
// existing TeKnowra databases, call providers, or infer a real employee identity.
func TestDeliveryIsolatedRealLangflow(t *testing.T) {
	path := os.Getenv("TEKNOWRA_DELIVERY_IDENTITY_FILE")
	if path == "" {
		t.Skip("explicit isolated identity file required")
	}
	var identity struct {
		Runner struct {
			APIKey string `json:"api_key"`
			ID     string `json:"id"`
		} `json:"runner"`
		FlowID string `json:"runtime_flow_id"`
		NodeID string `json:"model_node_id"`
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("isolated identity unavailable")
	}
	if json.Unmarshal(raw, &identity) != nil || identity.Runner.APIKey == "" || identity.FlowID == "" {
		t.Fatal("isolated identity incomplete")
	}
	ctx := context.Background()
	client, err := langflowapi.New("http://127.0.0.1:17861", identity.Runner.APIKey)
	if err != nil {
		t.Fatal(err)
	}
	flow, err := client.GetFlow(ctx, identity.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := langflowrelease.Digest(flow.Data)
	if err != nil {
		t.Fatal(err)
	}
	model := &deliveryModel{}
	root := filepath.Dir(filepath.Dir(path))
	var active struct {
		ReleaseID string `json:"release_id"`
	}
	raw, err = os.ReadFile(filepath.Join(root, "installed/current.json"))
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(raw, &active) != nil || active.ReleaseID == "" {
		t.Fatal("active release missing")
	}
	b := langflowdef.Binding{AgentID: "delivery-synthetic-agent", FlowID: identity.FlowID, WorkspaceID: 1,
		Models: []string{model.GetModelID()}, Nodes: []langflowdef.NodeBinding{{ID: identity.NodeID, Kind: "model"}},
		ReleaseID: active.ReleaseID, GraphDigest: digest, RuntimeOwnerID: identity.Runner.ID}
	o := hostcapability.Options{FlowURL: "http://127.0.0.1:17861", ServiceAPIKey: identity.Runner.APIKey, RequireRelease: true,
		Revalidate:   func(context.Context) (types.SearchTargets, error) { return nil, nil },
		ResolveModel: func(context.Context, string) (chat.Chat, error) { return model, nil },
		VerifyRelease: func(context.Context) error {
			_, err := langflowrelease.LocalComponents(ctx, client,
				filepath.Join(root, "installed/releases", active.ReleaseID, "payload"), os.Getenv("TEKNOWRA_DELIVERY_LIBRARY"))
			return err
		}}
	engine, err := New(b, o)
	if err != nil {
		t.Fatal(err)
	}
	state, err := engine.Execute(ctx, "delivery-"+uuid.NewString(), uuid.NewString(), "Return the synthetic host model marker.", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !state.IsComplete || state.FinalAnswer != "DELIVERY-HOST-MODEL-OK" || model.calls != 1 {
		t.Fatal("unexpected real runtime result")
	}
	evidence := map[string]any{"bundle": active.ReleaseID, "flow_id": identity.FlowID, "success": state.IsComplete, "model_calls": model.calls, "real_langflow": true, "production_identity": false, "external_provider_called": false}
	raw, _ = json.MarshalIndent(evidence, "", "  ")
	if err = os.WriteFile(filepath.Join(root, "execution-"+active.ReleaseID+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
