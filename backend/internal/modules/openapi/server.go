// Package openapi exposes the read-only public REST entry at /api/open/v1/*,
// authenticated by tenant-scoped API tokens (purpose openapi/both). It reuses
// the MCP token governance stack — SHA-256 hash lookup, expiry, revocation,
// three-layer rate limiting and per-call auditing — and registers GET routes
// only: no write operation is reachable through this entry.
package openapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/readonlyquery"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ratelimit"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

// tenantRateFactor mirrors the MCP entry: a tenant's aggregate budget is
// twice one token's budget, bounding multi-token amplification.
const tenantRateFactor = 2

// authFailRPS / authFailBurst bound rejected-credential attempts per client IP.
const (
	authFailRPS   = 1
	authFailBurst = 10
)

// ctxToken is the gin context key carrying the authenticated token row.
const ctxToken = "openapi_token"

// Deps wires the Open API entry into existing services.
type Deps struct {
	DB         *gorm.DB
	Tokens     *mcptoken.Service
	Exceptions *orderexception.Service
	// Audits records one row per endpoint call; a failed audit write rejects the
	// call (fail closed, same policy as the MCP entry). nil disables auditing.
	Audits *mcpaudit.Service
	// Redis, when set, backs the rate-limit buckets so the budget is shared
	// across replicas; nil keeps the in-process buckets.
	Redis redis.Scripter
	// RateRPS / RateBurst bound per-token request rates (fail closed on zero).
	RateRPS   float64
	RateBurst int
}

func (d *Deps) queries() *readonlyquery.Service {
	return &readonlyquery.Service{DB: d.DB, Exceptions: d.Exceptions}
}

// Register mounts GET /api/open/v1/* on the root router. Only read endpoints
// exist; requests carry `Authorization: Bearer <token>` where the token's
// purpose must be openapi or both.
func Register(r gin.IRouter, d *Deps) {
	if r == nil || d == nil || d.DB == nil || d.Tokens == nil {
		return
	}
	g := r.Group("/api/open/v1")
	g.Use(authMiddleware(d))
	h := &handlers{d: d}
	g.GET("/orders", h.audited("orders_list", h.ordersList))
	g.GET("/orders/:orderNo", h.audited("orders_detail", h.orderDetail))
	g.GET("/inventory", h.audited("inventory_list", h.inventoryList))
	g.GET("/reports/summary", h.audited("reports_summary", h.reportsSummary))
	g.GET("/exceptions", h.audited("exceptions_list", h.exceptionsList))
}

// authMiddleware authenticates the bearer token (purpose openapi/both) and
// applies the same three-layer rate limiting as the MCP entry: per token,
// per tenant aggregate, and a per-IP budget charged only on rejected
// credentials. Buckets are separate from the MCP buckets (own policy IDs) so
// one entry cannot starve the other.
func authMiddleware(d *Deps) gin.HandlerFunc {
	rps := d.RateRPS
	if rps <= 0 {
		rps = 5
	}
	burst := d.RateBurst
	if burst <= 0 {
		burst = 10
	}
	newLimiter := func(p ratelimit.Policy) ratelimit.Limiter {
		if d.Redis != nil {
			return ratelimit.NewRedisLimiter(d.Redis, p)
		}
		return ratelimit.NewLocalLimiter(p)
	}
	limiter := newLimiter(ratelimit.Policy{
		ID:        "openapi_readonly",
		Rate:      rate.Limit(rps),
		Burst:     burst,
		TTL:       10 * time.Minute,
		RetryHint: time.Second,
	})
	tenantLimiter := newLimiter(ratelimit.Policy{
		ID:        "openapi_readonly_tenant",
		Rate:      rate.Limit(rps * tenantRateFactor),
		Burst:     burst * tenantRateFactor,
		TTL:       10 * time.Minute,
		RetryHint: time.Second,
	})
	authFailLimiter := newLimiter(ratelimit.Policy{
		ID:        "openapi_readonly_authfail",
		Rate:      rate.Limit(authFailRPS),
		Burst:     authFailBurst,
		TTL:       10 * time.Minute,
		RetryHint: time.Second,
	})

	tooMany := func(c *gin.Context) {
		c.Header("Retry-After", "1")
		response.Fail(c, http.StatusTooManyRequests, response.CodeTooManyRequests, "rate limit exceeded")
		c.Abort()
	}

	return func(c *gin.Context) {
		clientKey := "ip:" + c.ClientIP()
		if !authFailLimiter.HasBudget(c.Request.Context(), clientKey) {
			tooMany(c)
			return
		}
		raw := bearerToken(c.GetHeader("Authorization"))
		if raw == "" {
			authFailLimiter.Allow(c.Request.Context(), clientKey)
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing bearer token")
			c.Abort()
			return
		}
		tok, err := d.Tokens.AuthenticateFor(c.Request.Context(), raw, mcptoken.PurposeOpenAPI)
		if err != nil {
			authFailLimiter.Allow(c.Request.Context(), clientKey)
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid or revoked token")
			c.Abort()
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
		c.Set(ctxToken, tok)
		c.Next()
	}
}

func bearerToken(header string) string {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func tokenOf(c *gin.Context) *mcptoken.Token {
	if v, ok := c.Get(ctxToken); ok {
		if tok, ok := v.(*mcptoken.Token); ok {
			return tok
		}
	}
	return nil
}

type handlers struct {
	d *Deps
}

// bufferedWriter holds the endpoint response in memory so it can still be
// withheld after the handler ran (used by the fail-closed audit trail). The
// endpoints answer with small JSON documents and never stream.
type bufferedWriter struct {
	gin.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *bufferedWriter) WriteHeader(code int)              { w.status = code }
func (w *bufferedWriter) WriteHeaderNow()                   {}
func (w *bufferedWriter) Write(b []byte) (int, error)       { return w.body.Write(b) }
func (w *bufferedWriter) WriteString(s string) (int, error) { return w.body.WriteString(s) }
func (w *bufferedWriter) Status() int                       { return w.status }
func (w *bufferedWriter) Size() int                         { return w.body.Len() }
func (w *bufferedWriter) Written() bool                     { return w.body.Len() > 0 }
func (w *bufferedWriter) Flush()                            {}

func (w *bufferedWriter) flushTo(dst gin.ResponseWriter) {
	dst.WriteHeader(w.status)
	if w.body.Len() > 0 {
		_, _ = dst.Write(w.body.Bytes())
	}
}

// audited wraps one endpoint with the shared per-call audit trail. Endpoint
// names live in the same log as the MCP tool names (prefixed "openapi:");
// request parameters and response payloads are never recorded. The response is
// buffered until its audit row is persisted: when the audit store is
// unavailable the query result is withheld and the call fails with 500, so no
// read ever completes without its audit row (same fail-closed policy as the
// MCP entry; the endpoints are read-only, so a rejected call is retryable).
func (h *handlers) audited(name string, fn gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		orig := c.Writer
		buf := &bufferedWriter{ResponseWriter: orig, status: http.StatusOK}
		c.Writer = buf
		fn(c)
		c.Writer = orig

		tok := tokenOf(c)
		if h.d.Audits == nil || tok == nil {
			buf.flushTo(orig)
			return
		}
		status := mcpaudit.StatusSuccess
		if buf.status >= 400 {
			status = mcpaudit.StatusError
		}
		if err := h.d.Audits.Write(c.Request.Context(), mcpaudit.WriteOpts{
			TenantID:    tok.TenantID,
			TokenID:     tok.ID,
			TokenName:   tok.Name,
			TokenMasked: tok.Masked(),
			Tool:        "openapi:" + name,
			Status:      status,
			DurationMs:  time.Since(start).Milliseconds(),
		}); err != nil {
			slog.Error("openapi_audit_write_failed", "endpoint", name, "error", err.Error())
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError,
				"audit log unavailable, request rejected")
			return
		}
		buf.flushTo(orig)
	}
}
