//go:build docker_integration

package sandbox

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// This test never runs a global sweep: it reads only its own marker and
// destroys only its own test session on cleanup.
func TestDockerRestoredBindingRenewsIdleMarkerIntegration(t *testing.T) {
	cfg := dockerIntegrationConfig(t)
	mgr := newDockerIntegrationManager(t, cfg)
	ctx := types.WithSandboxTenantID(context.Background(), dockerIntegrationTenantID)
	session := fmt.Sprintf("restore-marker-%d", time.Now().UnixNano())
	t.Cleanup(func() { require.NoError(t, mgr.DestroySession(ctx, session)) })
	handle, err := mgr.resolveSession(ctx, session)
	require.NoError(t, err)
	docker, err := NewDockerRemoteClient(cfg)
	require.NoError(t, err)
	docker.sweeper = nil
	_, err = docker.Exec(ctx, handle, RemoteExecRequest{Shell: true, Command: "touch -d '3 hours ago' " + dockerActivityMarker, Timeout: time.Second * 10})
	require.NoError(t, err)
	require.True(t, dockerActivityMarkerMTime(t, ctx, handle.ID()).Before(time.Now().Add(-2*time.Hour)))
	_, err = docker.Connect(ctx, RemoteConnectRequest{SandboxID: handle.ID()})
	require.NoError(t, err)
	require.WithinDuration(t, time.Now(), dockerActivityMarkerMTime(t, ctx, handle.ID()), 5*time.Second)
	result, err := mgr.ExecShellCommand(ctx, session, "echo restored-once", "/workspace", 10*time.Second, nil)
	require.NoError(t, err)
	require.True(t, result.IsSuccess())
	require.Contains(t, result.Stdout, "restored-once")
}

type restoreReclaimDockerClient struct {
	RemoteSandboxClient
	reclaimed bool
	commands  int
	preps     int
}

func (c *restoreReclaimDockerClient) Exec(ctx context.Context, h RemoteSandboxHandle, req RemoteExecRequest) (*RemoteExecResult, error) {
	if strings.Contains(req.Command, "mkdir -p") {
		c.preps++
		if !c.reclaimed {
			c.reclaimed = true
			// Real Engine deletion at the exact boundary observed in the failure.
			if err := c.RemoteSandboxClient.Delete(ctx, h.ID()); err != nil {
				return nil, err
			}
		}
	} else {
		c.commands++
	}
	return c.RemoteSandboxClient.Exec(ctx, h, req)
}

func TestDockerRestoredFirstSkillSurvivesReclamationIntegration(t *testing.T) {
	cfg := dockerIntegrationConfig(t)
	docker, err := NewDockerRemoteClient(cfg)
	require.NoError(t, err)
	docker.sweeper = nil
	store := NewMemorySessionSandboxBindingStore()
	deps := SessionBoundManagerConfig{Config: cfg, Client: docker, Store: store, Checker: PermissiveSessionExistenceChecker{}, ConfigID: "restore-first-skill", SkipHealthProbe: true}
	before, err := NewSessionBoundManager(deps)
	require.NoError(t, err)
	ctx := types.WithSandboxTenantID(context.Background(), dockerIntegrationTenantID)
	session := fmt.Sprintf("restore-first-skill-%d", time.Now().UnixNano())
	old, err := before.resolveSession(ctx, session)
	require.NoError(t, err)
	reclaim := &restoreReclaimDockerClient{RemoteSandboxClient: docker}
	deps.Client = reclaim
	after, err := NewSessionBoundManager(deps)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, after.DestroySession(ctx, session)) })
	command := `printf '%s' '{"amount":"100000","prepayment_percent":"60"}' | python /opt/weknora/tenant/skills/tyer-langflow-contract-evidence/scripts/report.py`
	result, err := after.ExecShellCommand(ctx, session, command, "/workspace", 30*time.Second, nil)
	require.NoError(t, err)
	require.True(t, result.IsSuccess(), "%+v", result)
	require.Contains(t, result.Stdout, "60000.00")
	require.True(t, reclaim.reclaimed)
	require.Equal(t, 2, reclaim.preps)
	require.Equal(t, 1, reclaim.commands, "business script must run exactly once")
	current, ok := after.BoundSandboxID(ctx, session)
	require.True(t, ok)
	require.NotEqual(t, old.ID(), current)
}
