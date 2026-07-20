package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"

	"github.com/xuri/excelize/v2"
)

func TestHandleExportPurchaseBillsXLSXIncludesBillsAndProductsWorksheets(t *testing.T) {
	helpers.APICache.Flush()
	oldDomain := config.BackendDomain
	t.Cleanup(func() {
		config.BackendDomain = oldDomain
		helpers.APICache.Flush()
	})

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/purchase_bill/all":
			_, _ = w.Write([]byte(`[{
				"id": 12,
				"supplier_sequence_number": 7001,
				"effective_date": "2026-05-01T00:00:00Z",
				"subtotal": "100",
				"total_vat": "15",
				"discount": "5",
				"total": "110",
				"state": 3
			}]`))
		case "/api/v2/purchase_bill/12":
			_, _ = w.Write([]byte(`{
				"id": 12,
				"products": [{
					"product_id": 9,
					"name": "Brake Pad",
					"part_name": "BP-100",
					"price": "50",
					"quantity": "2",
					"cost_price": "40",
					"shelf_number": "A-1",
					"discount": "0",
					"total_before_vat": "0"
				}],
				"manual_products": [{
					"name": "Manual Filter",
					"part_name": "MF-200",
					"price": "10",
					"quantity": "1",
					"cost_price": "7",
					"shelf_number": "B-2",
					"discount": "0",
					"total_before_vat": "10"
				}]
			}`))
		default:
			t.Fatalf("unexpected backend path %s", r.URL.Path)
		}

	}))
	t.Cleanup(backend.Close)
	config.BackendDomain = backend.URL

	config.SessionTokensMutex.Lock()
	config.SessionTokens["purchase-bill-export-test"] = "token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "purchase-bill-export-test")
		config.SessionTokensMutex.Unlock()
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/purchase-bills/export-xlsx", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "purchase-bill-export-test"})
	w := httptest.NewRecorder()

	HandleExportPurchaseBillsXLSX(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") {
		t.Fatalf("expected XLSX content type, got %q", got)
	}
	if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "purchase-bills.xlsx") {
		t.Fatalf("expected XLSX attachment, got %q", got)
	}

	workbook, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("open exported workbook: %v", err)
	}
	defer workbook.Close()

	if got := workbook.GetSheetList(); len(got) != 2 || got[0] != "Bills" || got[1] != "Products" {
		t.Fatalf("expected Bills and Products worksheets, got %v", got)
	}
	if got, err := workbook.GetCellValue("Bills", "A1"); err != nil || got != "مرجع الفاتورة" {
		t.Fatalf("expected Bills header, got %q (%v)", got, err)
	}
	if got, err := workbook.GetCellValue("Products", "A1"); err != nil || got != "مرجع الفاتورة" {
		t.Fatalf("expected Products header, got %q (%v)", got, err)
	}
	if got, err := workbook.GetCellValue("Products", "B2"); err != nil || got != "Brake Pad" {
		t.Fatalf("expected stock product row, got %q (%v)", got, err)
	}
	if got, err := workbook.GetCellValue("Products", "B3"); err != nil || got != "Manual Filter" {
		t.Fatalf("expected manual product row, got %q (%v)", got, err)
	}
	if got, err := workbook.GetCellValue("Products", "H2"); err != nil || got != "100" {
		t.Fatalf("expected calculated line total, got %q (%v)", got, err)
	}
}

func TestBuildSalesBillExportWorkbookUsesImportContract(t *testing.T) {
	helpers.APICache.Flush()
	oldDomain := config.BackendDomain
	t.Cleanup(func() {
		config.BackendDomain = oldDomain
		helpers.APICache.Flush()
	})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/bill/44" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"id": 44, "store_id": 1, "client_id": 7, "user_name": "Customer",
			"effective_date": "2026-05-01T00:00:00Z", "discount": "2",
			"products": [{"name":"Brake Pad","price":"50","quantity":"2","total_before_vat":"0"}],
			"manual_products": []
		}`))
	}))
	t.Cleanup(backend.Close)
	config.BackendDomain = backend.URL

	workbook, err := buildSalesBillExportWorkbook("token", []models.Invoice{{ID: 44}}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer workbook.Close()
	if got, _ := workbook.GetCellValue("Bills", "A1"); got != "Bill Reference" {
		t.Fatalf("Bills header = %q", got)
	}
	if got, _ := workbook.GetCellValue("Products", "B1"); got != "Product Name" {
		t.Fatalf("Products header = %q", got)
	}
	if got, _ := workbook.GetCellValue("Products", "E2"); got != "100" {
		t.Fatalf("line total = %q, want calculated 100", got)
	}
	headers, _ := workbook.GetRows("Products")
	for _, header := range headers[0] {
		if strings.Contains(strings.ToLower(header), "part number") {
			t.Fatalf("unexpected part-number header %q", header)
		}
	}
}
