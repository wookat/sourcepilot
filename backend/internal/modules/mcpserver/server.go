// Package mcpserver exposes a Model Context Protocol (MCP) entry backed by
// tenant-scoped API tokens. Query tools require the readonly scope; a small
// whitelist of write tools (R179 W1: order tagging) requires the write:ops
// scope plus the env-level and tenant-level write gates, and every write
// goes through dry-run → one-time confirmation → execute with fail-closed
// auditing. Message sending / external-platform actions are permanently
// excluded from this surface.
package mcpserver

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redis/go-redis/v9"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpwrite"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ratelimit"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

// serverName identifies this MCP server to clients.
const serverName = "sourcepilot-readonly"

// tenantRateFactor is how much aggregate budget one tenant gets relative to a
// single token, bounding the gain from spreading traffic over several tokens.
const tenantRateFactor = 2

// authFailRPS / authFailBurst bound rejected-credential attempts per client IP.
const (
	authFailRPS   = 1
	authFailBurst = 10
)

// Deps wires the MCP entry into existing services.
type Deps struct {
	DB         *gorm.DB
	Tokens     *mcptoken.Service
	Exceptions *orderexception.Service
	// Audits records one row per tool call; a failed audit write rejects the
	// call (fail closed). nil disables auditing.
	Audits *mcpaudit.Service
	// Redis, when set, backs the rate-limit buckets so the budget is shared
	// across replicas; nil keeps the in-process buckets.
	Redis redis.Scripter
	// RateRPS / RateBurst bound per-token request rates (fail closed on zero).
	RateRPS   float64
	RateBurst int
	// Version reported to MCP clients (optional).
	Version string
	// WriteEnabled mirrors MCP_WRITE_ENABLED (default false). When false the
	// write tools are not registered at all, whatever the token scope.
	WriteEnabled bool
	// Orders backs the whitelisted order-tag write tools; nil disables them.
	Orders *order.Service
	// Procurement backs the whitelisted purchase-order write tools
	// (mark-placed / 物流回填); nil disables them.
	Procurement *procurement.Service
	// Settings resolves the tenant-level write switch; nil keeps every
	// tenant's write gate closed (fail closed).
	Settings *settings.Service
}

// writes builds the governed write pipeline for one request.
func (d *Deps) writes() *mcpwrite.Service {
	return &mcpwrite.Service{
		DB:     d.DB,
		Gate:   &mcpwrite.Gate{EnvEnabled: d.WriteEnabled, Settings: d.Settings},
		Audits: d.Audits,
	}
}

// GinHandler returns the POST /api/mcp handler: tenant API-token auth,
// per-token rate limiting, then a stateless streamable MCP session whose
// tools are bound to the authenticated tenant.
func GinHandler(d *Deps) gin.HandlerFunc {
	if d == nil || d.Tokens == nil || d.DB == nil {
		return func(c *gin.Context) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeInternalError, "mcp entry unavailable")
		}
	}
	rps := d.RateRPS
	if rps <= 0 {
		rps = 5
	}
	burst := d.RateBurst
	if burst <= 0 {
		burst = 10
	}
	// With Redis available the buckets are shared across replicas; otherwise
	// they are per-process (budget multiplies by replica count, documented in
	// docs/mcp.md).
	newLimiter := func(p ratelimit.Policy) ratelimit.Limiter {
		if d.Redis != nil {
			return ratelimit.NewRedisLimiter(d.Redis, p)
		}
		return ratelimit.NewLocalLimiter(p)
	}
	limiter := newLimiter(ratelimit.Policy{
		ID:        "mcp_readonly",
		Rate:      rate.Limit(rps),
		Burst:     burst,
		TTL:       10 * time.Minute,
		RetryHint: time.Second,
	})
	// A tenant may hold several tokens, each with its own bucket; this bucket
	// caps what one tenant can consume in aggregate.
	tenantLimiter := newLimiter(ratelimit.Policy{
		ID:        "mcp_readonly_tenant",
		Rate:      rate.Limit(rps * tenantRateFactor),
		Burst:     burst * tenantRateFactor,
		TTL:       10 * time.Minute,
		RetryHint: time.Second,
	})
	// Authentication failures are bounded per client IP: the per-token bucket
	// can only apply after a token resolves, so unauthenticated callers would
	// otherwise get unlimited token-hash lookups.
	authFailLimiter := newLimiter(ratelimit.Policy{
		ID:        "mcp_readonly_authfail",
		Rate:      rate.Limit(authFailRPS),
		Burst:     authFailBurst,
		TTL:       10 * time.Minute,
		RetryHint: time.Second,
	})

	streamable := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		tok, ok := tokenFromContext(r.Context())
		if !ok || tok == nil {
			return nil
		}
		srv := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: d.Version}, nil)
		// Scope axes are independent: readonly grants the query tools,
		// write:ops grants the write whitelist (and nothing else). The env
		// gate hides write tools entirely; the tenant gate is enforced per
		// call inside the write pipeline.
		if tok.HasScope(mcptoken.ScopeReadonly) {
			registerTools(srv, d, tok.TenantID)
		}
		if d.WriteEnabled && tok.HasScope(mcptoken.ScopeWriteOps) {
			registerWriteTools(srv, d, tok)
		}
		if d.Audits != nil {
			srv.AddReceivingMiddleware(auditMiddleware(d.Audits, tok))
		}
		return srv
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// The SDK's localhost/Origin check targets browser DNS-rebinding on
		// localhost-bound dev servers. This entry sits behind the API gateway
		// and requires a tenant API token on every request, so the check is
		// disabled to keep Docker/reverse-proxy Host headers working.
		DisableLocalhostProtection: true,
	})

	tooMany := func(c *gin.Context) {
		c.Header("Retry-After", "1")
		response.Fail(c, http.StatusTooManyRequests, response.CodeTooManyRequests, "rate limit exceeded")
	}

	// auditReject records entry-level 401/429 events (throttled per source and
	// minute); tok is nil when the caller is unauthenticated (row under tenant 0).
	auditReject := func(c *gin.Context, key, status string, tok *mcptoken.Token) {
		if d.Audits == nil {
			return
		}
		opts := mcpaudit.WriteOpts{Tool: "mcp:auth", Status: status}
		if tok != nil {
			opts.TenantID = tok.TenantID
			opts.TokenID = tok.ID
			opts.TokenName = tok.Name
			opts.TokenMasked = tok.Masked()
		}
		if err := d.Audits.WriteThrottled(c.Request.Context(), key, opts); err != nil {
			slog.Warn("mcp_auth_audit_write_failed", "status", status, "error", err.Error())
		}
	}

	return func(c *gin.Context) {
		clientKey := "ip:" + c.ClientIP()
		// The failure budget is only charged for rejected credentials, so valid
		// traffic is never throttled by it; it is checked first so a client that
		// burned its budget stops costing token lookups.
		if !authFailLimiter.HasBudget(c.Request.Context(), clientKey) {
			auditReject(c, clientKey, mcpaudit.StatusRateLimited, nil)
			tooMany(c)
			return
		}
		raw := bearerToken(c.GetHeader("Authorization"))
		if raw == "" {
			authFailLimiter.Allow(c.Request.Context(), clientKey)
			auditReject(c, clientKey, mcpaudit.StatusAuthFailed, nil)
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing bearer token")
			return
		}
		tok, err := d.Tokens.Authenticate(c.Request.Context(), raw)
		if err != nil {
			authFailLimiter.Allow(c.Request.Context(), clientKey)
			auditReject(c, clientKey, mcpaudit.StatusAuthFailed, nil)
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid or revoked token")
			return
		}
		if !limiter.Allow(c.Request.Context(), tok.ID.String()).Allowed {
			auditReject(c, "token:"+tok.ID.String(), mcpaudit.StatusRateLimited, tok)
			tooMany(c)
			return
		}
		if !tenantLimiter.Allow(c.Request.Context(), strconv.FormatInt(tok.TenantID, 10)).Allowed {
			auditReject(c, "tenant:"+strconv.FormatInt(tok.TenantID, 10), mcpaudit.StatusRateLimited, tok)
			tooMany(c)
			return
		}
		d.Tokens.TouchLastUsed(c.Request.Context(), tok.ID)
		ctx := withToken(c.Request.Context(), tok)
		streamable.ServeHTTP(c.Writer, c.Request.WithContext(ctx))
	}
}

// auditMiddleware appends one audit row per tools/call: tenant, token, tool
// name, outcome and duration. Arguments and results are never recorded. When
// the audit row cannot be persisted the successful result is withheld and the
// call fails, so no tool call ever completes without its audit row (audit
// completeness is chosen over availability; tools are read-only, so a
// rejected call is safely retryable).
func auditMiddleware(audits *mcpaudit.Service, tok *mcptoken.Token) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			toolName := ""
			if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok && p != nil {
				toolName = p.Name
			}
			if isWriteTool(toolName) {
				// Write tools audit inside the write pipeline (dry_run /
				// execute rows with params/result summaries, committed in the
				// same transaction as the mutation); a second generic row here
				// would double-count them. Calls refused before the pipeline
				// runs (parameter validation, or the tool not being registered
				// for a token without write:ops) audit here instead, so no
				// tools/call — least of all one probing the write whitelist —
				// escapes the trail.
				wctx, sig := mcpwrite.WithSignal(ctx)
				start := time.Now()
				res, err := next(wctx, method, req)
				if sig.Reached() {
					return res, err
				}
				if werr := audits.Write(ctx, mcpaudit.WriteOpts{
					TenantID:      tok.TenantID,
					TokenID:       tok.ID,
					TokenName:     tok.Name,
					TokenMasked:   tok.Masked(),
					Tool:          toolName,
					Status:        mcpaudit.StatusError,
					ResultSummary: "rejected before write pipeline",
					DurationMs:    time.Since(start).Milliseconds(),
				}); werr != nil {
					slog.Error("mcp_tool_audit_write_failed", "tool", toolName, "error", werr.Error())
					if err == nil {
						return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "audit log unavailable, tool call rejected"}
					}
				}
				return res, err
			}
			start := time.Now()
			res, err := next(ctx, method, req)
			status := mcpaudit.StatusSuccess
			if err != nil {
				status = mcpaudit.StatusError
			} else if ctr, ok := res.(*mcp.CallToolResult); ok && ctr != nil && ctr.IsError {
				status = mcpaudit.StatusError
			}
			if werr := audits.Write(ctx, mcpaudit.WriteOpts{
				TenantID:    tok.TenantID,
				TokenID:     tok.ID,
				TokenName:   tok.Name,
				TokenMasked: tok.Masked(),
				Tool:        toolName,
				Status:      status,
				DurationMs:  time.Since(start).Milliseconds(),
			}); werr != nil {
				slog.Error("mcp_tool_audit_write_failed", "tool", toolName, "error", werr.Error())
				if err == nil {
					return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "audit log unavailable, tool call rejected"}
				}
			}
			return res, err
		}
	}
}

func bearerToken(header string) string {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
