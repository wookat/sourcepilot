package mcpwrite

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
)

// Tenant-level write switch, stored in the settings table (group mcp, item
// write_enabled). Settings PUT is admin-only, so only tenant admins can open
// their gate; the default (absent row) is off.
const (
	SettingsGroupMCP       = "mcp"
	SettingsKeyWriteEnable = "write_enabled"
)

// Gate rejections (403 semantics on the MCP surface).
var (
	// ErrWriteDisabledGlobal: the deployment-level kill switch
	// MCP_WRITE_ENABLED is off (default).
	ErrWriteDisabledGlobal = errors.New("mcp write: 写操作未开启（全局 MCP_WRITE_ENABLED=false）")
	// ErrWriteDisabledTenant: the tenant has not opted in via settings
	// mcp/write_enabled (default off).
	ErrWriteDisabledTenant = errors.New("mcp write: 写操作未开启（租户级开关关闭）")
)

// Gate evaluates the env-level and tenant-level write gates. The third gate
// (token write:ops scope) is enforced by the MCP server before write tools
// are even registered, and re-checked by the orchestrator.
type Gate struct {
	// EnvEnabled mirrors MCP_WRITE_ENABLED (default false).
	EnvEnabled bool
	// Settings resolves the tenant-level switch. A nil service or a lookup
	// error closes the gate (fail closed).
	Settings *settings.Service
}

// Check returns nil only when both the env gate and the tenant gate are
// open. Any lookup failure closes the gate.
func (g *Gate) Check(ctx context.Context, tenantID int64) error {
	if g == nil || !g.EnvEnabled {
		return ErrWriteDisabledGlobal
	}
	if g.Settings == nil {
		return ErrWriteDisabledTenant
	}
	vals, err := g.Settings.PlainByGroup(ctx, tenantID, SettingsGroupMCP)
	if err != nil {
		return fmt.Errorf("%w（租户开关读取失败）", ErrWriteDisabledTenant)
	}
	if !strings.EqualFold(strings.TrimSpace(vals[SettingsKeyWriteEnable]), "true") {
		return ErrWriteDisabledTenant
	}
	return nil
}
