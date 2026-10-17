package langflowcatalog

import (
	"context"
	"testing"
	"time"
)

func TestDirectoryTicketIsBoundedAndRevalidatesEveryRead(t *testing.T) {
	allowed := true
	now := time.Now()
	sessions := NewSessions(func(context.Context, string) (Catalog, error) {
		if !allowed {
			return Catalog{}, context.Canceled
		}
		return Catalog{AgentID: "agent", Models: []Resource{{ID: "m", Name: "模型"}}}, nil
	})
	sessions.now = func() time.Time { return now }
	token, _, err := sessions.Create(context.Background(), "agent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sessions.Read(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	allowed = false
	if _, err = sessions.Read(context.Background(), token); err == nil {
		t.Fatal("revoked editor retained directory")
	}
	allowed = true
	now = now.Add(16 * time.Minute)
	if _, err = sessions.Read(context.Background(), token); err == nil {
		t.Fatal("expired directory accepted")
	}
	if _, err = sessions.Read(context.Background(), "foreign"); err == nil {
		t.Fatal("foreign ticket accepted")
	}
}
