package mcpwrite

import (
	"context"
	"sync/atomic"
)

// Reaching the write pipeline is signalled back to the MCP entry layer so it
// can tell "the pipeline audited this call" from "the call was refused before
// the pipeline ran" (parameter validation, tool not registered for the
// caller's scope). Every Run path writes an audit row, so the signal is set
// once Run owns the call.

type signalKey struct{}

// Signal reports whether a tools/call reached the write pipeline.
type Signal struct{ reached atomic.Bool }

// Reached is true when the write pipeline took over the call (and therefore
// audited it).
func (s *Signal) Reached() bool { return s != nil && s.reached.Load() }

// WithSignal returns a context carrying a fresh pipeline signal.
func WithSignal(ctx context.Context) (context.Context, *Signal) {
	sig := &Signal{}
	return context.WithValue(ctx, signalKey{}, sig), sig
}

func markReached(ctx context.Context) {
	if sig, ok := ctx.Value(signalKey{}).(*Signal); ok && sig != nil {
		sig.reached.Store(true)
	}
}
