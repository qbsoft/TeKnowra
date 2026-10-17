package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// Simulate the daemon reclaiming a restored sandbox after resolution but
// before workspace preparation. The business command must still run once.
type reclaimedBeforePrepClient struct {
	*fakeRemoteClient
	remaining  int
	preps      int
	commands   int
	commandErr error
	prepErr    error
}

func (c *reclaimedBeforePrepClient) Exec(ctx context.Context, h RemoteSandboxHandle, req RemoteExecRequest) (*RemoteExecResult, error) {
	if strings.Contains(req.Command, "mkdir -p") {
		c.preps++
		if c.prepErr != nil {
			return nil, c.prepErr
		}
		if c.remaining > 0 {
			c.remaining--
			if err := c.fakeRemoteClient.Delete(ctx, h.ID()); err != nil {
				return nil, err
			}
			return nil, NewRemoteError(SandboxTypeDocker, "Exec", RemoteErrorKindNotFound, "container reclaimed before preparation", nil)
		}
	} else {
		c.commands++
		if c.commandErr != nil {
			return nil, c.commandErr
		}
	}
	return c.fakeRemoteClient.Exec(ctx, h, req)
}

func (c *reclaimedBeforePrepClient) ListDir(_ context.Context, h RemoteSandboxHandle, dir string) ([]RemoteDirEntry, error) {
	size := int64(100)
	if h.ID() != "docker-1" {
		size = 200 + int64(c.commands)
	}
	return []RemoteDirEntry{{Path: dir + "/report.txt", Type: RemoteEntryFile, Size: size}}, nil
}

func restoredPrepHarness(t *testing.T, losses int) (*SessionBoundManager, *reclaimedBeforePrepClient, context.Context) {
	t.Helper()
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(10000))
	fake := newFakeRemoteClient(SandboxTypeDocker)
	fake.omitsInboundTokenCarrier = true
	cfg := DefaultConfig()
	cfg.Type, cfg.DockerImage = SandboxTypeDocker, "restored-skill-image"
	store := NewMemorySessionSandboxBindingStore()
	deps := SessionBoundManagerConfig{Config: cfg, Client: fake, Store: store, Checker: &fakeSessionExistenceChecker{exists: true}, SkipHealthProbe: true}
	before, err := NewSessionBoundManager(deps)
	require.NoError(t, err)
	_, err = before.resolveSession(ctx, "restored")
	require.NoError(t, err)
	// A fresh manager restores the persisted binding, not an in-memory handle.
	client := &reclaimedBeforePrepClient{fakeRemoteClient: fake, remaining: losses}
	deps.Client = client
	after, err := NewSessionBoundManager(deps)
	require.NoError(t, err)
	return after, client, ctx
}

func TestRestoredDockerShellRecoversBeforeCommandStarts(t *testing.T) {
	mgr, client, ctx := restoredPrepHarness(t, 1)
	result, err := mgr.ExecShellCommand(ctx, "restored", "echo business-once", "/workspace", time.Second, nil)
	require.NoError(t, err)
	require.Zero(t, result.ExitCode)
	require.Equal(t, 1, client.commands)
	require.Equal(t, 2, client.preps)
	require.Equal(t, 2, client.createCount)
}

func TestRestoredDockerScriptRecoversBeforeCommandStarts(t *testing.T) {
	mgr, client, ctx := restoredPrepHarness(t, 1)
	_, err := mgr.Execute(ctx, &ExecuteConfig{SessionID: "restored", Script: "report.py", ScriptContent: "print(60000)", SkipValidation: true})
	require.NoError(t, err)
	require.Equal(t, 1, client.commands)
	require.Equal(t, 2, client.preps)
}

func TestRestoredDockerPreparationRecoveryIsBounded(t *testing.T) {
	mgr, client, ctx := restoredPrepHarness(t, 3)
	_, err := mgr.ExecShellCommand(ctx, "restored", "echo must-not-run", "/workspace", time.Second, nil)
	require.Error(t, err)
	require.Equal(t, 2, client.preps)
	require.Zero(t, client.commands)
}

func TestRestoredDockerDoesNotReplayStartedCommand(t *testing.T) {
	mgr, client, ctx := restoredPrepHarness(t, 0)
	client.commandErr = NewRemoteError(SandboxTypeDocker, "Exec", RemoteErrorKindNotFound, "container vanished after command started", nil)
	result, err := mgr.ExecShellCommand(ctx, "restored", "echo business-once", "/workspace", time.Second, nil)
	require.NoError(t, err) // Shell reports execution failures through ExecuteResult.
	require.NotEmpty(t, result.Error)
	require.Equal(t, 1, client.commands)
	require.Equal(t, 1, client.createCount)
}

func TestRestoredDockerMaintenanceNeverRebinds(t *testing.T) {
	mgr, client, ctx := restoredPrepHarness(t, 1)
	h, err := mgr.resolveSession(ctx, "restored")
	require.NoError(t, err)
	_, err = mgr.ExecShellCommandWithOptions(ctx, "restored", "echo must-not-run", ShellExecOptions{
		ExpectedSandboxID: h.ID(), WorkDir: "/workspace", Timeout: time.Second,
	})
	require.Error(t, err)
	require.Zero(t, client.commands)
	require.Equal(t, 1, client.createCount)
}

func TestRestoredDockerInstallNeverRebinds(t *testing.T) {
	mgr, client, ctx := restoredPrepHarness(t, 1)
	_, err := mgr.ExecShellCommandWithOptions(ctx, "restored", "echo must-not-run", ShellExecOptions{AsRoot: true, WorkDir: "/workspace"})
	require.Error(t, err)
	require.Zero(t, client.commands)
	require.Equal(t, 1, client.createCount)
}

func TestRestoredDockerAmbiguousPreparationFailureKeepsBinding(t *testing.T) {
	for _, kind := range []RemoteErrorKind{RemoteErrorKindAuthentication, RemoteErrorKindTimeout, RemoteErrorKindUnavailable, RemoteErrorKindInternal, RemoteErrorKindInvalidRequest} {
		t.Run(string(kind), func(t *testing.T) {
			mgr, client, ctx := restoredPrepHarness(t, 0)
			client.prepErr = NewRemoteError(SandboxTypeDocker, "Exec", kind, "preparation failure", nil)
			_, err := mgr.ExecShellCommand(ctx, "restored", "echo must-not-run", "/workspace", time.Second, nil)
			require.Error(t, err)
			require.Equal(t, 1, client.preps)
			require.Equal(t, 1, client.createCount)
			require.Zero(t, client.commands)
		})
	}
}

func TestRestoredDockerOutputSnapshotDiscardsOldHandle(t *testing.T) {
	mgr, client, ctx := restoredPrepHarness(t, 1)
	result, snapshot, err := mgr.ExecShellCommandWithOutputSnapshot(ctx, "restored", "echo business-once", ShellExecOptions{}, SessionOutputRoot)
	require.NoError(t, err)
	require.True(t, result.IsSuccess())
	require.NotNil(t, snapshot)
	require.Equal(t, int64(200), snapshot.Before[0].Size, "old container's same-name file (100 bytes) must not leak into the new snapshot")
	require.Equal(t, int64(201), snapshot.After[0].Size)
	require.Equal(t, 1, client.commands)
}
