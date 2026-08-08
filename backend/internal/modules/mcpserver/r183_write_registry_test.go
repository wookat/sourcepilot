package mcpserver

import "testing"

// Every whitelisted write tool must be recognised by isWriteTool, otherwise
// the entry-level audit middleware writes a second, mode-less row for it.
func TestIsWriteToolCoversWholeWhitelist(t *testing.T) {
	for _, name := range []string{
		ToolOrdersAddTag,
		ToolOrdersRemoveTag,
		ToolExceptionsMark,
		ToolProcurementMarkPlaced,
		ToolProcurementFillLogistics,
		ToolProcurementMarkPaid,
	} {
		if !isWriteTool(name) {
			t.Errorf("isWriteTool(%q) = false, want true", name)
		}
	}
	// The exported list (audit read API least-exposure filter) must agree
	// with isWriteTool, so a new write action cannot be registered in one
	// place only.
	names := WriteToolNames()
	if len(names) != 6 {
		t.Errorf("WriteToolNames() has %d entries, want 6", len(names))
	}
	for _, name := range names {
		if !isWriteTool(name) {
			t.Errorf("WriteToolNames entry %q not recognised by isWriteTool", name)
		}
	}
	if isWriteTool("orders_list") {
		t.Error("isWriteTool must not claim read tools")
	}
}
