package permmatrix

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Persona keys (matrix.json columns).
const (
	personaAdmin         = "admin"
	personaOperator      = "operator"
	personaReadonly      = "readonly"
	personaCrossTenant   = "crossTenantAdmin"
	personaPlatformAdmin = "platformAdmin"
)

// Expectation classes.
const (
	expectAllow  = "allow"  // must not be rejected by auth/permission (401/403)
	expectForbid = "forbid" // must be rejected with 403 by a route/handler guard
)

var personaOrder = []string{personaReadonly, personaOperator, personaCrossTenant, personaAdmin}

// optionalPersonas are only probed on routes that declare an expectation for
// them. platformAdmin (tenant 0 admin) is registered on the platform tenant
// management routes; other routes are not probed with it by default.
var optionalPersonas = []string{personaPlatformAdmin}

// routeEntry is one row of the permission matrix registry.
type routeEntry struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	// Public routes are reachable without a Bearer token.
	Public bool `json:"public,omitempty"`
	// Probe=false routes are registered but not probed by the generic
	// matrix runner (e.g. self-mutating auth routes that would invalidate
	// fixture tokens). They must document why in Note and be covered by
	// module-level tests.
	Probe *bool `json:"probe,omitempty"`
	// Personas maps persona key -> expectation class.
	Personas map[string]string `json:"personas,omitempty"`
	Note     string            `json:"note,omitempty"`
}

func (e routeEntry) probeEnabled() bool { return e.Probe == nil || *e.Probe }

//go:embed matrix.json
var matrixRaw []byte

func loadMatrix(t *testing.T) []routeEntry {
	t.Helper()
	var entries []routeEntry
	require.NoError(t, json.Unmarshal(matrixRaw, &entries))
	return entries
}

// TestRouteRegistryComplete enforces that every route mounted on the
// production router is registered in matrix.json with explicit expectations,
// and that matrix.json contains no stale routes. Adding a new endpoint
// without registering its permission expectations fails this test.
func TestRouteRegistryComplete(t *testing.T) {
	h := sharedHarness(t)
	entries := loadMatrix(t)

	registered := map[string]routeEntry{}
	for _, e := range entries {
		registered[routeKey(e.Method, e.Path)] = e
	}

	var missing, stale []string
	mounted := map[string]bool{}
	for _, r := range h.registeredRoutes() {
		key := routeKey(r.Method, r.Path)
		mounted[key] = true
		if _, ok := registered[key]; !ok {
			missing = append(missing, key)
		}
	}
	for key := range registered {
		if !mounted[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("routes mounted but not registered in matrix.json (add entries with reviewed permission expectations, see docs/permission-matrix.md):\n  %s", strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("matrix.json entries no longer mounted (remove them):\n  %s", strings.Join(stale, "\n  "))
	}

	if os.Getenv("PERM_MATRIX_GENERATE") == "1" && len(missing) > 0 {
		generateEntries(t, h, entries, missing)
	}

	for _, e := range entries {
		if e.Public || !e.probeEnabled() {
			continue
		}
		for _, p := range personaOrder {
			exp := e.Personas[p]
			require.Containsf(t, []string{expectAllow, expectForbid}, exp,
				"%s %s: persona %q must declare %q or %q", e.Method, e.Path, p, expectAllow, expectForbid)
		}
		for _, p := range optionalPersonas {
			if exp, ok := e.Personas[p]; ok {
				require.Containsf(t, []string{expectAllow, expectForbid}, exp,
					"%s %s: persona %q must declare %q or %q", e.Method, e.Path, p, expectAllow, expectForbid)
			}
		}
	}
}

// TestAnonymousRequiresAuth asserts every non-public route rejects
// unauthenticated requests with 401.
func TestAnonymousRequiresAuth(t *testing.T) {
	h := sharedHarness(t)
	for _, e := range loadMatrix(t) {
		if e.Public || !e.probeEnabled() {
			continue
		}
		w := h.do(t, e.Method, fillPathParams(e.Path), "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: anonymous expected 401, got %d", e.Method, e.Path, w.Code)
		}
	}
}

// TestPermissionMatrix runs every registered route as every persona and
// asserts the declared expectation class:
//   - allow: response is not 401/403 (business errors like 400/404 are fine —
//     the request passed authentication and authorization guards)
//   - forbid: response is exactly 403
func TestPermissionMatrix(t *testing.T) {
	h := sharedHarness(t)
	for _, e := range loadMatrix(t) {
		if e.Public || !e.probeEnabled() {
			continue
		}
		path := fillPathParams(e.Path)
		personas := personaOrder
		for _, pk := range optionalPersonas {
			if _, ok := e.Personas[pk]; ok {
				personas = append(append([]string{}, personas...), pk)
			}
		}
		for _, pk := range personas {
			exp := e.Personas[pk]
			w := h.do(t, e.Method, path, h.Personas[pk].Token)
			switch exp {
			case expectForbid:
				if w.Code != http.StatusForbidden {
					t.Errorf("%s %s [%s]: expected 403 (guard), got %d body=%s", e.Method, e.Path, pk, w.Code, truncateBody(w.Body.String()))
				}
			case expectAllow:
				if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
					t.Errorf("%s %s [%s]: expected pass-through (non-401/403), got %d body=%s", e.Method, e.Path, pk, w.Code, truncateBody(w.Body.String()))
				}
			}
		}
	}
}

func truncateBody(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 160 {
		return s[:160] + "..."
	}
	return s
}

// generateEntries probes unregistered routes and prints draft matrix.json
// entries based on observed behavior. Drafts MUST be security-reviewed before
// committing: observed "allow" on a write route for readonly usually means a
// missing guard (fix the code, not the expectation).
func generateEntries(t *testing.T, h *harness, existing []routeEntry, missing []string) {
	t.Helper()
	drafts := make([]routeEntry, 0, len(missing))
	for _, key := range missing {
		parts := strings.SplitN(key, " ", 2)
		method, path := parts[0], parts[1]
		e := routeEntry{Method: method, Path: path, Personas: map[string]string{}}
		for _, pk := range personaOrder {
			w := h.do(t, method, fillPathParams(path), h.Personas[pk].Token)
			if w.Code == http.StatusForbidden {
				e.Personas[pk] = expectForbid
			} else if w.Code == http.StatusUnauthorized {
				e.Personas[pk] = "OBSERVED_401_REVIEW"
			} else {
				e.Personas[pk] = expectAllow
			}
		}
		drafts = append(drafts, e)
	}
	all := append(append([]routeEntry{}, existing...), drafts...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].Path != all[j].Path {
			return all[i].Path < all[j].Path
		}
		return all[i].Method < all[j].Method
	})
	out, err := json.MarshalIndent(all, "", "  ")
	require.NoError(t, err)
	fmt.Printf("PERM_MATRIX_GENERATE draft matrix.json:\n%s\n", string(out))
}
