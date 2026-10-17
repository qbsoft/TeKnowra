package service

import (
	"context"
	"fmt"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Never substitute a publisher's directory for the caller's personal snapshot.
func ensureLangflowCallerDirectory(ctx context.Context, metadata interfaces.MCPMetadataService, tenant uint64, service string, load func(context.Context) error, check func(context.Context) error, required ...string) error {
	complete := func(snapshot *types.MCPMetadata) bool {
		if snapshot == nil || snapshot.Stale {
			return false
		}
		for _, name := range required {
			found := false
			for _, tool := range snapshot.Tools {
				if tool != nil && tool.Name == name {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	if err := check(ctx); err != nil {
		return err
	}
	snapshot, err := metadata.GetMCPMetadata(ctx, tenant, service)
	if err != nil {
		return err
	}
	if complete(snapshot) {
		return nil
	}
	if err = load(ctx); err != nil {
		return err
	}
	if err = check(ctx); err != nil {
		return err
	}
	snapshot, err = metadata.GetMCPMetadata(ctx, tenant, service)
	if err != nil {
		return err
	}
	if !complete(snapshot) {
		return fmt.Errorf("当前用户MCP目录缺少所需工具或已失效，请检查本人授权")
	}
	return nil
}
