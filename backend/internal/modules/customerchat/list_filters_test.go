package customerchat

import "testing"

func TestParseTriBoolQuery(t *testing.T) {
	if v := parseTriBoolQuery("1"); v == nil || !*v {
		t.Fatalf("1 should be true")
	}
	if v := parseTriBoolQuery("true"); v == nil || !*v {
		t.Fatalf("true should be true")
	}
	if v := parseTriBoolQuery("0"); v == nil || *v {
		t.Fatalf("0 should be false, got %v", v)
	}
	if v := parseTriBoolQuery("false"); v == nil || *v {
		t.Fatalf("false should be false, got %v", v)
	}
	if v := parseTriBoolQuery(""); v != nil {
		t.Fatalf("empty should be nil")
	}
	if v := parseTriBoolQuery("abc"); v != nil {
		t.Fatalf("garbage should be nil")
	}
}
