package migrationimport

import "testing"

func TestTemplateSampleMatchesFieldCount(t *testing.T) {
	for _, kind := range []string{KindProduct, KindOrder, KindInventory, KindSource} {
		if got, want := len(templateSample(kind)), len(FieldsForKind(kind)); got != want {
			t.Fatalf("kind %s: sample has %d values, want %d (one per field)", kind, got, want)
		}
	}
}
