// Package mcpserver exposes a read-only Model Context Protocol (MCP) entry
// backed by tenant-scoped API tokens. Only query tools are registered; no
// write operation is reachable through this endpoint.
package mcpserver

import (
	"net/http"
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

	return func(c *gin.Context) {
		raw := bearerToken(c.GetHeader("Authorization"))
		if raw == "" {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing bearer token")
			return
		}
		tok, err := d.Tokens.Authenticate(c.Request.Context(), raw)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid or revoked token")
			return
		}
		dec := limiter.Allow(c.Request.Context(), tok.ID.String())
		if !dec.Allowed {
			c.Header("Retry-After", "1")
			response.Fail(c, http.StatusTooManyRequests, response.CodeBadRequest, "rate limit exceeded")
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
