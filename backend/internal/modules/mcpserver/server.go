// Package mcpserver exposes a read-only Model Context Protocol (MCP) entry
// backed by tenant-scoped API tokens. Only query tools are registered; no
// write operation is reachable through this endpoint.
package mcpserver

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
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
	// RateRPS / RateBurst bound per-token request rates (fail closed on zero).
	RateRPS   float64
	RateBurst int
	// Version reported to MCP clients (optional).
	Version string
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
	limiter := ratelimit.NewLocalLimiter(ratelimit.Policy{
		ID:        "mcp_readonly",
		Rate:      rate.Limit(rps),
		Burst:     burst,
		TTL:       10 * time.Minute,
		RetryHint: time.Second,
	})
	// A tenant may hold several tokens, each with its own bucket; this bucket
	// caps what one tenant can consume in aggregate.
	tenantLimiter := ratelimit.NewLocalLimiter(ratelimit.Policy{
		ID:        "mcp_readonly_tenant",
		Rate:      rate.Limit(rps * tenantRateFactor),
		Burst:     burst * tenantRateFactor,
		TTL:       10 * time.Minute,
		RetryHint: time.Second,
	})
	// Authentication failures are bounded per client IP: the per-token bucket
	// can only apply after a token resolves, so unauthenticated callers would
	// otherwise get unlimited token-hash lookups.
	authFailLimiter := ratelimit.NewLocalLimiter(ratelimit.Policy{
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
		registerTools(srv, d, tok.TenantID)
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

	return func(c *gin.Context) {
		clientKey := "ip:" + c.ClientIP()
		// The failure budget is only charged for rejected credentials, so valid
		// traffic is never throttled by it; it is checked first so a client that
		// burned its budget stops costing token lookups.
		if !authFailLimiter.HasBudget(clientKey) {
			tooMany(c)
			return
		}
		raw := bearerToken(c.GetHeader("Authorization"))
		if raw == "" {
			authFailLimiter.Allow(c.Request.Context(), clientKey)
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing bearer token")
			return
		}
		tok, err := d.Tokens.Authenticate(c.Request.Context(), raw)
		if err != nil {
			authFailLimiter.Allow(c.Request.Context(), clientKey)
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid or revoked token")
			return
		}
		if !limiter.Allow(c.Request.Context(), tok.ID.String()).Allowed {
			tooMany(c)
			return
		}
		if !tenantLimiter.Allow(c.Request.Context(), strconv.FormatInt(tok.TenantID, 10)).Allowed {
			tooMany(c)
			return
		}
		d.Tokens.TouchLastUsed(c.Request.Context(), tok.ID)
		ctx := withToken(c.Request.Context(), tok)
		streamable.ServeHTTP(c.Writer, c.Request.WithContext(ctx))
	}
}

func bearerToken(header string) string {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
