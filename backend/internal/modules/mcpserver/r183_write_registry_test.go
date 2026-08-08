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
	if isWriteTool("orders_list") {
		t.Error("isWriteTool must not claim read tools")
	}
}
