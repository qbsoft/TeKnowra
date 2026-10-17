package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type langflowCallerDirectoryStub struct {
	interfaces.MCPMetadataService
	snapshot *types.MCPMetadata
	seen     []types.Principal
}

func TestLangflowSharedMCPRefreshesDirectoryMissingPublishedTool(t *testing.T) {
	m := &langflowCallerDirectoryStub{snapshot: &types.MCPMetadata{Tools: []*types.MCPTool{{Name: "find_customer"}}}}
	calls := 0
	err := ensureLangflowCallerDirectory(context.Background(), m, 10000, "crm", func(context.Context) error {
		calls++
		m.snapshot.Tools = append(m.snapshot.Tools, &types.MCPTool{Name: "check_customer_access"})
		return nil
	}, func(context.Context) error { return nil }, "find_customer", "check_customer_access")
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestLangflowSharedMCPRejectsStillMissingToolAndRevocationDuringRefresh(t *testing.T) {
	m := &langflowCallerDirectoryStub{snapshot: &types.MCPMetadata{}}
	err := ensureLangflowCallerDirectory(context.Background(), m, 10000, "crm", func(context.Context) error { return nil }, func(context.Context) error { return nil }, "check_customer_access")
	require.Error(t, err)
	checks := 0
	err = ensureLangflowCallerDirectory(context.Background(), m, 10000, "crm", func(context.Context) error {
		m.snapshot.Tools = []*types.MCPTool{{Name: "check_customer_access"}}
		return nil
	}, func(context.Context) error {
		checks++
		if checks > 1 {
			return fmt.Errorf("share revoked during OAuth")
		}
		return nil
	}, "check_customer_access")
	require.ErrorContains(t, err, "share revoked during OAuth")
}

func (s *langflowCallerDirectoryStub) GetMCPMetadata(ctx context.Context, tenant uint64, id string) (*types.MCPMetadata, error) {
	s.seen = append(s.seen, types.MCPOAuthPrincipalFromContext(ctx))
	return s.snapshot, nil
}

func TestLangflowSharedMCPBootstrapsOnlyCurrentCallerDirectory(t *testing.T) {
	ctx := types.WithPrincipal(context.Background(), types.Principal{Type: types.PrincipalWebUser, ID: "sales01"})
	m := &langflowCallerDirectoryStub{}
	loads, checks := 0, 0
	err := ensureLangflowCallerDirectory(ctx, m, 10000, "crm", func(current context.Context) error {
		loads++
		require.Equal(t, types.MCPOAuthPrincipalFromContext(ctx), types.MCPOAuthPrincipalFromContext(current))
		m.snapshot = &types.MCPMetadata{TenantID: 10000, ServiceID: "crm", Principal: "web_user:sales01"}
		return nil
	}, func(context.Context) error { checks++; return nil })
	require.NoError(t, err)
	require.Equal(t, 1, loads)
	require.GreaterOrEqual(t, checks, 2)
	require.Len(t, m.seen, 2)
	for _, p := range m.seen {
		require.Equal(t, "sales01", p.ID)
	}
}

func TestLangflowSharedMCPDirectoryDenialNeverFallsBack(t *testing.T) {
	m := &langflowCallerDirectoryStub{}
	loads := 0
	err := ensureLangflowCallerDirectory(context.Background(), m, 10000, "crm", func(context.Context) error { loads++; return nil }, func(context.Context) error { return fmt.Errorf("share revoked") })
	require.ErrorContains(t, err, "share revoked")
	require.Zero(t, loads)
	err = ensureLangflowCallerDirectory(context.Background(), m, 10000, "crm", func(context.Context) error { return fmt.Errorf("OAuth denied") }, func(context.Context) error { return nil })
	require.ErrorContains(t, err, "OAuth denied")
	require.Nil(t, m.snapshot)
}

func TestLangflowSharedMCPDirectoryRequiresCompleteFreshSnapshot(t *testing.T) {
	m := &langflowCallerDirectoryStub{snapshot: &types.MCPMetadata{Stale: true}}
	err := ensureLangflowCallerDirectory(context.Background(), m, 10000, "crm", func(context.Context) error { return nil }, func(context.Context) error { return nil })
	require.Error(t, err)
	m.snapshot = &types.MCPMetadata{TenantID: 10000, ServiceID: "crm"}
	err = ensureLangflowCallerDirectory(context.Background(), m, 10000, "crm", func(context.Context) error { t.Fatal("fresh personal directory should not reconnect"); return nil }, func(context.Context) error { return nil })
	require.NoError(t, err)
}
