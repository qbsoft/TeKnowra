package langflowruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/einoruntime"
	"github.com/Tencent/WeKnora/internal/agent/hostcapability"
	"github.com/Tencent/WeKnora/internal/agent/runbudget"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestDualEngineAdmissionBeforeInvocation(t *testing.T) {
	t.Setenv("TEKNOWRA_AGENT_MAX_CONCURRENT", "1")
	release, err := runbudget.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	for name, invoke := range map[string]func() error{
		"eino": func() error {
			_, err := (&einoruntime.Engine{}).Execute(context.Background(), "s", "m", "q", nil)
			return err
		},
		"langflow": func() error { _, err := (&Engine{}).Execute(context.Background(), "s", "m", "q", nil); return err },
	} {
		if err := invoke(); !errors.Is(err, runbudget.ErrCapacity) {
			t.Fatalf("%s bypassed shared gate: %v", name, err)
		}
	}
	release()
	// Both real Execute entry points must release on errors before any provider call.
	_, err = (&einoruntime.Engine{}).Execute(context.Background(), "s", "m", "q", []chat.Message{{Content: "history"}})
	if err == nil {
		t.Fatal("Eino invalid history unexpectedly accepted")
	}
	lf := &Engine{opts: hostcapability.Options{Revalidate: func(context.Context) (types.SearchTargets, error) { return nil, errors.New("scope denied") }}}
	_, err = lf.Execute(context.Background(), "s", "m", "q", nil)
	if err == nil {
		t.Fatal("Langflow denied scope unexpectedly accepted")
	}
	final, err := runbudget.Acquire(context.Background())
	if err != nil {
		t.Fatal("engine error leaked shared slot", err)
	}
	final()
}
