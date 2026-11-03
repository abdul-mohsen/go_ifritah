package handlers

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSaleImportPreviewKeepsDuplicateCodesInASet checks the one duplicate test of the preview script: its codes are a Set, and the
// script asks the Set with has().
func TestSaleImportPreviewKeepsDuplicateCodesInASet(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	source, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "..", "static", "js", "sale-import-preview.js"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(source)
	if !strings.Contains(script, "DUPLICATE_CODES = new Set([") || !strings.Contains(script, "DUPLICATE_CODES.has(warning.code)") {
		t.Error("the duplicate codes of the sales preview should be a Set, checked with DUPLICATE_CODES.has()")
	}
}
