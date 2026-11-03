package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"afrita/config"
	"afrita/helpers"
)

// enableSalesImport turns sales imports on for one test. They are off in production.
func enableSalesImport(t *testing.T) {
	t.Helper()
	previous := salesImportEnabled
	salesImportEnabled = true
	t.Cleanup(func() { salesImportEnabled = previous })
}

// TestSalesImportIsRefusedWhileItIsOff checks that each sales import answers 503 and posts no bill, even with a valid
// session.
func TestSalesImportIsRefusedWhileItIsOff(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	body, err := json.Marshal(importRequest{Type: "bills", StoreID: 1, Mapping: map[string]int{"part_name": 0, "price": 1}, Rows: [][]string{{"Brake Pad", "75"}}, ImportSession: importTestSession})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(t *testing.T) *httptest.ResponseRecorder{
		"xlsx": func(t *testing.T) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			HandleImportBillsXLSX(w, multipartUpload(t, cookie, "/api/bill-import/upload?type=sales", "file", "sales.xlsx", importWorkbook(t, billImportSales), map[string]string{importSessionField: importTestSession}))
			return w
		},
		"csv": func(t *testing.T) *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			HandleImportBillsUpload(w, multipartUpload(t, cookie, "/api/invoices/import-csv", "csv_file", "bills.csv", []byte("name,price\nBrake Pad,75\n"), map[string]string{"store_id": "1", importSessionField: importTestSession}))
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

// startPurchaseBackend fakes the purchase bill route and counts the purchase bills that it receives.
func startPurchaseBackend(t *testing.T) *atomic.Int32 {
	t.Helper()
	var created atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/purchase_bill" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		created.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(backend.Close)
	previous := config.BackendDomain
	config.BackendDomain = backend.URL
	helpers.APICache.Flush()
	t.Cleanup(func() {
		config.BackendDomain = previous
		helpers.APICache.Flush()
	})
	return &created
}

// TestPurchaseImportsAreNotRefusedWhileSalesImportIsOff checks that the purchase imports still post their bills while
// sales imports are off.
func TestPurchaseImportsAreNotRefusedWhileSalesImportIsOff(t *testing.T) {
	created := startPurchaseBackend(t)
	cookie := seedSaleSession(t)

	w := httptest.NewRecorder()
	HandleImportBillsXLSX(w, multipartUpload(t, cookie, "/api/bill-import/upload?type=purchase", "file", "purchase.xlsx", importWorkbook(t, billImportPurchase), nil))
	if w.Code != http.StatusOK || created.Load() != 2 {
		t.Fatalf("xlsx purchase status = %d purchase bills = %d body=%s, want 200 and 2", w.Code, created.Load(), w.Body.String())
	}

	body, err := json.Marshal(importRequest{Type: "purchase_bills", StoreID: 1, Mapping: map[string]int{"part_name": 0, "price": 1, "quantity": 2}, Rows: [][]string{{"Brake Pad", "75", "2"}, {"Engine Oil", "40", "1"}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/data-import/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	HandleDataImportExecute(w, req)
	if w.Code != http.StatusOK || created.Load() != 4 {
		t.Fatalf("data import purchase status = %d purchase bills = %d body=%s, want 200 and 4", w.Code, created.Load(), w.Body.String())
	}
}
