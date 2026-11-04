package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// enableSalesImport turns sales imports on for one test. They are off in production.
func enableSalesImport(t *testing.T) {
	t.Helper()
	previous := salesImportEnabled
	salesImportEnabled = true
	t.Cleanup(func() { salesImportEnabled = previous })
}

// TestSalesImportIsRefusedWhileItIsOff checks that each sales import answers 503 and posts no bill.
func TestSalesImportIsRefusedWhileItIsOff(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	body, err := json.Marshal(importRequest{Type: "bills", StoreID: 1, Mapping: map[string]int{"part_name": 0, "price": 1}, Rows: [][]string{{"Brake Pad", "75"}}})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(t *testing.T) *httptest.ResponseRecorder{
		"xlsx": func(t *testing.T) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			HandleImportBillsXLSX(w, multipartUpload(t, cookie, "/api/bill-import/upload?type=sales", "file", "sales.xlsx", importWorkbook(t, billImportSales), nil))
			return w
		},
		"csv": func(t *testing.T) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			HandleImportBillsUpload(w, multipartUpload(t, cookie, "/api/invoices/import-csv", "csv_file", "bills.csv", []byte("name,price\nBrake Pad,75\n"), map[string]string{"store_id": "1"}))
			return w
		},
		"data import": func(t *testing.T) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodPost, "/api/data-import/execute", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			HandleDataImportExecute(w, req)
			return w
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			w := run(t)
			if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), salesImportOffMessage) {
				t.Fatalf("status = %d body=%s, want 503 with the off message", w.Code, w.Body.String())
			}
		})
	}
	if got := len(writes.all()); got != 0 {
		t.Fatalf("refused sales imports posted %d bills, want 0", got)
	}
}

// TestPurchaseImportsAreNotRefusedWhileSalesImportIsOff checks that the purchase imports still post their bills while
// sales imports are off.
func TestPurchaseImportsAreNotRefusedWhileSalesImportIsOff(t *testing.T) {
	writes := startPurchaseCapture(t)
	cookie := seedSaleSession(t)

	w := httptest.NewRecorder()
	HandleImportBillsXLSX(w, multipartUpload(t, cookie, "/api/bill-import/upload?type=purchase", "file", "purchase.xlsx", importWorkbook(t, billImportPurchase), nil))
	if w.Code != http.StatusOK || len(writes.all()) != 2 {
		t.Fatalf("xlsx purchase status = %d purchase bills = %d body=%s, want 200 and 2", w.Code, len(writes.all()), w.Body.String())
	}

	result := runPurchaseImport(t, cookie, [][]string{
		{"Brake Pad", "75", "2", "1600", "40", "2026-11-03"},
		{"Engine Oil", "40", "1", "1601", "25", "2026-11-03"},
	})
	if result.Success != 2 || len(writes.all()) != 4 {
		t.Fatalf("data import purchase success = %d purchase bills = %d (results %+v), want 2 and 4", result.Success, len(writes.all()), result.Results)
	}
}

// importWorkbook returns the import template of one bill type.
func importWorkbook(t *testing.T, importType string) []byte {
	t.Helper()
	workbook, err := buildBillImportTemplate(importType, false, billImportTemplateOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	var xlsx bytes.Buffer
	if err := workbook.Write(&xlsx); err != nil {
		t.Fatal(err)
	}
	return xlsx.Bytes()
}
