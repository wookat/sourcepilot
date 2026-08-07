package openapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
)

// R160 P2 regression (R159 audit): invalid enum / boolean query parameters
// answer 400 instead of being silently degraded (empty result set or
// unfiltered list), matching the date and pagination policy.
func TestInvalidEnumAndBoolParamsAnswer400(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens := newTestServer(t, db, 100, 100)
	tok, err := tokens.Create(context.Background(), 1, "t1", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	bad := []string{
		"/api/open/v1/exceptions?severity=error",
		"/api/open/v1/exceptions?severity=bogus",
		"/api/open/v1/exceptions?exceptionType=bogus",
		"/api/open/v1/inventory?lowStockOnly=notabool",
		"/api/open/v1/inventory?lowStockOnly=yes",
		"/api/open/v1/orders?status=bogus",
		"/api/open/v1/orders?paymentStatus=bogus",
	}
	for _, path := range bad {
		res, body := get(t, srv, path, tok.Plaintext)
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: want 400 got %d", path, res.StatusCode)
		}
		if code, _ := body["code"].(float64); code != 40001 {
			t.Fatalf("%s: want business code 40001, got %v", path, body["code"])
		}
	}

	// Absent and valid values keep working. (Exception listing itself needs
	// the full exception service and is covered by the permission matrix
	// suite; this server wires Exceptions=nil.)
	good := []string{
		"/api/open/v1/inventory?lowStockOnly=true",
		"/api/open/v1/inventory?lowStockOnly=false",
		"/api/open/v1/inventory?lowStockOnly=1",
		"/api/open/v1/inventory?lowStockOnly=0",
		"/api/open/v1/orders?status=pending&paymentStatus=unpaid",
	}
	for _, path := range good {
		res, _ := get(t, srv, path, tok.Plaintext)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: want 200 got %d", path, res.StatusCode)
		}
	}
}
