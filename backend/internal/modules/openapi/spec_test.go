package openapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/openapi"
	"github.com/trademind-ai/trademind/backend/internal/modules/readonlyquery"
)

const specPath = "../../../../docs/openapi/open-api.v1.json"

type spec struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]schemaDef `json:"schemas"`
	} `json:"components"`
}

type schemaDef struct {
	Type       string                     `json:"type"`
	Properties map[string]json.RawMessage `json:"properties"`
	AllOf      []schemaDef                `json:"allOf"`
	Ref        string                     `json:"$ref"`
}

func loadSpec(t *testing.T) spec {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(specPath))
	if err != nil {
		t.Fatalf("read OpenAPI spec: %v", err)
	}
	var s spec
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse OpenAPI spec: %v", err)
	}
	return s
}

// TestSpecRoutesMatchImplementation fails when the OpenAPI 3 spec and the
// registered gin routes drift apart (either direction).
func TestSpecRoutesMatchImplementation(t *testing.T) {
	s := loadSpec(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := openTestDB(t)
	openapi.Register(r, &openapi.Deps{DB: db, Tokens: &mcptoken.Service{DB: db}})

	var implemented []string
	for _, route := range r.Routes() {
		implemented = append(implemented, route.Method+" "+route.Path)
	}
	var declared []string
	for path, methods := range s.Paths {
		ginPath := path
		for strings.Contains(ginPath, "{") {
			open := strings.Index(ginPath, "{")
			close := strings.Index(ginPath, "}")
			ginPath = ginPath[:open] + ":" + ginPath[open+1:close] + ginPath[close+1:]
		}
		for method := range methods {
			declared = append(declared, strings.ToUpper(method)+" "+ginPath)
		}
	}
	sort.Strings(implemented)
	sort.Strings(declared)
	if !reflect.DeepEqual(implemented, declared) {
		t.Fatalf("OpenAPI spec and implementation drifted:\n spec: %v\n impl: %v", declared, implemented)
	}
	for _, route := range implemented {
		if !strings.HasPrefix(route, "GET ") {
			t.Fatalf("read-only surface must only register GET routes, got %q", route)
		}
	}
}

// specFields flattens a schema's property names, following allOf composition
// and local $ref targets.
func specFields(t *testing.T, s spec, def schemaDef) []string {
	t.Helper()
	var out []string
	if def.Ref != "" {
		name := strings.TrimPrefix(def.Ref, "#/components/schemas/")
		target, ok := s.Components.Schemas[name]
		if !ok {
			t.Fatalf("unresolved $ref %q", def.Ref)
		}
		return specFields(t, s, target)
	}
	for _, sub := range def.AllOf {
		out = append(out, specFields(t, s, sub)...)
	}
	for name := range def.Properties {
		out = append(out, name)
	}
	return out
}

// dtoFields lists the JSON field names of a response DTO.
func dtoFields(t *testing.T, v any) []string {
	t.Helper()
	var out []string
	tp := reflect.TypeOf(v)
	for i := 0; i < tp.NumField(); i++ {
		f := tp.Field(i)
		if f.Anonymous {
			out = append(out, dtoFields(t, reflect.New(f.Type).Elem().Interface())...)
			continue
		}
		tag := strings.Split(f.Tag.Get("json"), ",")[0]
		if tag == "" || tag == "-" {
			t.Fatalf("field %s.%s lacks a json tag", tp.Name(), f.Name)
		}
		out = append(out, tag)
	}
	return out
}

// TestSpecSchemasMatchDTOs fails when a response schema and its Go DTO drift.
func TestSpecSchemasMatchDTOs(t *testing.T) {
	s := loadSpec(t)
	cases := map[string]any{
		"OrderSummary":     readonlyquery.OrderSummary{},
		"OrderItemSummary": readonlyquery.OrderItemSummary{},
		"OrderDetail":      readonlyquery.OrderDetailOut{},
		"InventoryItem":    readonlyquery.InventoryItem{},
		"CurrencySales":    readonlyquery.CurrencySales{},
		"ReportSummary":    readonlyquery.ReportSummaryOut{},
		"ExceptionItem":    readonlyquery.ExceptionItem{},
	}
	for name, dto := range cases {
		def, ok := s.Components.Schemas[name]
		if !ok {
			t.Fatalf("schema %q missing from spec", name)
		}
		want := dtoFields(t, dto)
		got := specFields(t, s, def)
		sort.Strings(want)
		sort.Strings(got)
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("schema %q drifted from DTO:\n spec: %v\n dto:  %v", name, got, want)
		}
	}
}
