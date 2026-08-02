package permmatrix

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/api"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	tenantA int64 = 910052
	tenantB int64 = 910053
)

type persona struct {
	Name     string
	Token    string
	UserID   uuid.UUID
	TenantID int64
}

type harness struct {
	Router   *gin.Engine
	DB       *gorm.DB
	Cfg      *config.Config
	Keys     *auth.KeySet
	Personas map[string]*persona
	// ShopGranted is a tenant A shop granted to the operator persona;
	// ShopUngranted is a tenant A shop the operator has no grant for.
	ShopGranted   uuid.UUID
	ShopUngranted uuid.UUID
}

var (
	harnessOnce sync.Once
	harnessVal  *harness
	harnessErr  error
)

// sharedHarness builds the full production router (api.Register) against the
// isolated PostgreSQL test database once per test binary.
func sharedHarness(t *testing.T) *harness {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	dbCfg, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping permission matrix contract tests (see docs/permission-matrix.md)")
	}
	harnessOnce.Do(func() {
		harnessVal, harnessErr = buildHarness(dbCfg.URL)
	})
	require.NoError(t, harnessErr)
	return harnessVal
}

func buildHarness(dbURL string) (*harness, error) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	if err := database.AutoMigrate(db); err != nil {
		return nil, err
	}

	cfg := &config.Config{
		AppEnv:                  "test",
		JWTSecret:               "perm-matrix-suite-jwt-secret-with-safe-length",
		JWTExpHrs:               1,
		MasterKey:               "perm-matrix-master-key-0123456789abcdef",
		CollectorTimeoutSeconds: 1,
		CollectorBaseURL:        "http://127.0.0.1:59321", // intentionally unreachable
		EnableDemoSeed:          true,
	}
	keys, err := auth.BuildKeySet(cfg)
	if err != nil {
		return nil, err
	}

	engine := gin.New()
	engine.Use(middleware.RequestID())
	api.Register(engine, &api.Deps{Config: cfg, DB: db, MigrationsReady: true})

	h := &harness{Router: engine, DB: db, Cfg: cfg, Keys: keys, Personas: map[string]*persona{}}
	if err := h.seedPersonas(); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *harness) seedShop(tenantID int64, name string) (uuid.UUID, error) {
	s := &shop.Shop{TenantID: tenantID, Platform: "manual", ShopName: name, Status: "active", AuthStatus: "authorized"}
	if err := h.DB.Create(s).Error; err != nil {
		return uuid.Nil, err
	}
	return s.ID, nil
}

func (h *harness) seedPersonas() error {
	// Remove fixtures from previous runs so the suite is repeatable against a
	// persistent test database.
	if err := h.DB.Exec("DELETE FROM user_store_permissions WHERE user_id IN (SELECT id FROM admin_users WHERE password_hash = 'permission-matrix-fixture')").Error; err != nil {
		return err
	}
	if err := h.DB.Exec("DELETE FROM admin_users WHERE password_hash = 'permission-matrix-fixture'").Error; err != nil {
		return err
	}
	if err := h.DB.Exec("DELETE FROM shops WHERE shop_name LIKE 'perm-matrix-%'").Error; err != nil {
		return err
	}

	specs := []struct {
		key    string
		role   string
		tenant int64
	}{
		{personaAdmin, adminperm.RoleAdmin, tenantA},
		{personaOperator, adminperm.RoleOperator, tenantA},
		{personaReadonly, adminperm.RoleReadonly, tenantA},
		{personaCrossTenant, adminperm.RoleAdmin, tenantB},
	}
	for _, s := range specs {
		id := uuid.New()
		username := admin.NewInternalUsername()
		u := &admin.AdminUser{
			Base:         model.Base{ID: id},
			TenantID:     s.tenant,
			Username:     username,
			PasswordHash: "permission-matrix-fixture",
			Role:         s.role,
			Status:       admin.StatusActive,
		}
		if err := h.DB.Create(u).Error; err != nil {
			return err
		}
		token, _, err := auth.MintAccessToken(h.Cfg, h.Keys, auth.MintAccessInput{
			UserID: id, Username: username, TenantID: s.tenant, TokenVersion: 1,
		})
		if err != nil {
			return err
		}
		h.Personas[s.key] = &persona{Name: s.key, Token: token, UserID: id, TenantID: s.tenant}
	}

	var err error
	if h.ShopGranted, err = h.seedShop(tenantA, "perm-matrix-granted"); err != nil {
		return err
	}
	if h.ShopUngranted, err = h.seedShop(tenantA, "perm-matrix-ungranted"); err != nil {
		return err
	}
	grant := &admin.UserStorePermission{
		ID:              uuid.New(),
		UserID:          h.Personas[personaOperator].UserID,
		StoreID:         h.ShopGranted,
		Platform:        "manual",
		PermissionScope: admin.StorePermScopeOperate,
	}
	return h.DB.Create(grant).Error
}

// fillPathParams replaces gin path params with syntactically valid values that
// do not reference any existing resource.
func fillPathParams(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		switch {
		case p == "":
			continue
		case strings.HasPrefix(p, "*"):
			parts[i] = "matrix-probe"
		case strings.HasPrefix(p, ":"):
			parts[i] = pathParamValue(strings.TrimPrefix(p, ":"))
		}
	}
	return strings.Join(parts, "/")
}

func pathParamValue(name string) string {
	switch name {
	case "platform":
		return "douyin_shop"
	case "source", "provider", "providerKey":
		return "1688"
	case "group", "groupKey":
		return "system"
	case "key", "itemKey", "profileKey":
		return "matrix-probe"
	case "code":
		return "matrix-probe"
	case "date":
		return "2026-01-01"
	default:
		return uuid.NewString()
	}
}

func (h *harness) do(t *testing.T, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	return h.doBody(t, method, path, token, "{}")
}

func (h *harness) doBody(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "perm-matrix-"+uuid.NewString())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.Router.ServeHTTP(w, req)
	return w
}

// routeKey joins method and registered path, e.g. "GET /api/v1/products".
func routeKey(method, path string) string { return method + " " + path }

// registeredRoutes returns every route mounted on the production router.
func (h *harness) registeredRoutes() []gin.RouteInfo {
	return h.Router.Routes()
}
