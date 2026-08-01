package orderexception

import (
	"testing"

	"github.com/google/uuid"
)

func boolPtr(v bool) *bool { return &v }

func TestFilterAggRowsAllView(t *testing.T) {
	openID := uuid.New()
	handledID := uuid.New()
	ignoredID := uuid.New()
	rows := []aggRow{
		{exceptionType: TypeSKUUnmatched, sourceType: SourceOrderItemSKUMatch, sourceID: openID},
		{exceptionType: TypeSKUUnmatched, sourceType: SourceOrderItemSKUMatch, sourceID: handledID},
		{exceptionType: TypeSKUUnmatched, sourceType: SourceOrderItemSKUMatch, sourceID: ignoredID},
	}
	marks := map[string]markPair{
		markKey(TypeSKUUnmatched, SourceOrderItemSKUMatch, handledID.String()): {handled: true},
		markKey(TypeSKUUnmatched, SourceOrderItemSKUMatch, ignoredID.String()): {ignored: true},
	}

	if got := filterAggRows(rows, marks, ListOrderExceptionsRequest{}); len(got) != 1 {
		t.Fatalf("default open view: want 1 row, got %d", len(got))
	}
	if got := filterAggRows(rows, marks, ListOrderExceptionsRequest{Handled: boolPtr(true)}); len(got) != 1 {
		t.Fatalf("handled view: want 1 row, got %d", len(got))
	}
	if got := filterAggRows(rows, marks, ListOrderExceptionsRequest{Ignored: boolPtr(true)}); len(got) != 1 {
		t.Fatalf("ignored view: want 1 row, got %d", len(got))
	}
	if got := filterAggRows(rows, marks, ListOrderExceptionsRequest{All: boolPtr(true)}); len(got) != 3 {
		t.Fatalf("all view: want 3 rows, got %d", len(got))
	}
}
