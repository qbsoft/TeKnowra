package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type clientOnlyRepo struct {
	interfaces.MCPOAuthRepository
	client *types.MCPOAuthClient
	err    error
}

func (r *clientOnlyRepo) GetClient(context.Context, uint64, string) (*types.MCPOAuthClient, error) {
	return r.client, r.err
}

const (
	registered = "https://teknowra.example.com/api/v1/mcp-oauth/callback"
	fromLAN    = "http://192.168.0.224:8080/api/v1/mcp-oauth/callback"
)

func TestCanonicalRedirectURI(t *testing.T) {
	tests := []struct {
		name        string
		repo        *clientOnlyRepo
		externalURL string
		supplied    string
		want        string
	}{
		{
			name:     "登记过：不管用户这次从哪个地址进来，都用登记时的地址",
			repo:     &clientOnlyRepo{client: &types.MCPOAuthClient{RedirectURI: registered}},
			supplied: fromLAN,
			want:     registered,
		},
		{
			name:        "登记过：连配置的对外地址也不能盖过登记的——对方只认登记的那个",
			repo:        &clientOnlyRepo{client: &types.MCPOAuthClient{RedirectURI: registered}},
			externalURL: "https://other.example.com",
			supplied:    fromLAN,
			want:        registered,
		},
		{
			name:        "没登记过、配了对外地址：用配置的，第一次登记就登记对",
			repo:        &clientOnlyRepo{},
			externalURL: "https://teknowra.example.com/",
			supplied:    fromLAN,
			want:        registered,
		},
		{
			name:     "没登记过、也没配置：与上游行为一致，用前端报的",
			repo:     &clientOnlyRepo{},
			supplied: fromLAN,
			want:     fromLAN,
		},
		{
			name:     "查登记记录出错：不猜，退回前端报的",
			repo:     &clientOnlyRepo{err: errors.New("db down")},
			supplied: fromLAN,
			want:     fromLAN,
		},
		{
			name:     "登记记录里地址是空的：当作没登记",
			repo:     &clientOnlyRepo{client: &types.MCPOAuthClient{RedirectURI: "  "}},
			supplied: fromLAN,
			want:     fromLAN,
		},
		{
			name:        "对外地址配得不像样：不拿坏地址去登记",
			repo:        &clientOnlyRepo{},
			externalURL: "teknowra.example.com",
			supplied:    fromLAN,
			want:        fromLAN,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_EXTERNAL_URL", tt.externalURL)
			m := &OAuthManager{repo: tt.repo}
			if got := m.CanonicalRedirectURI(context.Background(), 10000, "svc", tt.supplied); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCanonicalRedirectURINilSafe(t *testing.T) {
	var m *OAuthManager
	if got := m.CanonicalRedirectURI(context.Background(), 1, "svc", fromLAN); got != fromLAN {
		t.Errorf("nil manager must fall back to the supplied URI, got %q", got)
	}
}
