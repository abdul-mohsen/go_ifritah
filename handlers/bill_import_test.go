package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"
	"afrita/helpers"

	"github.com/xuri/excelize/v2"
)

// TestBuildBillImportTemplateAppliesOverrides is a regression test for a
// real e2e failure: the generated template's example Store ID/Supplier ID/
// Branch ID are illustrative placeholders (they only need to look right,
// not belong to any real account), so re-uploading the raw template as-is
// gets rejected by the backend's store/supplier ownership checks for
// whichever account downloaded it. HandleDownloadBillImportTemplate accepts
// optional query overrides so a caller (or an e2e test) can request a
// template pre-filled with IDs that actually belong to their account.
func TestBuildBillImportTemplateAppliesOverrides(t *testing.T) {
	t.Run("purchase: overrides store/supplier ids on every example bill row", func(t *testing.T) {
		workbook, err := buildBillImportTemplate(billImportPurchase, false, billImportTemplateOverrides{
			StoreID: "42", SupplierID: "7",
		})
		if err != nil {
			t.Fatal(err)
		}
		defer workbook.Close()
		for _, row := range []string{"2", "3"} {
			if got, _ := workbook.GetCellValue("Bills", "B"+row); got != "42" {
				t.Fatalf("Bills!B%s store id = %q, want 42", row, got)
			}
			if got, _ := workbook.GetCellValue("Bills", "C"+row); got != "7" {
				t.Fatalf("Bills!C%s supplier id = %q, want 7", row, got)
			}
		}
	})

	t.Run("sales: overrides store/branch/customer ids on every example bill row", func(t *testing.T) {
		workbook, err := buildBillImportTemplate(billImportSales, false, billImportTemplateOverrides{
			StoreID: "42", BranchID: "3", CustomerID: "99",
		})
		if err != nil {
			t.Fatal(err)
		}
		defer workbook.Close()
		for _, row := range []string{"2", "3"} {
			if got, _ := workbook.GetCellValue("Bills", "B"+row); got != "42" {
				t.Fatalf("Bills!B%s store id = %q, want 42", row, got)
			}
			if got, _ := workbook.GetCellValue("Bills", "C"+row); got != "3" {
				t.Fatalf("Bills!C%s branch id = %q, want 3", row, got)
			}
			if got, _ := workbook.GetCellValue("Bills", "D"+row); got != "99" {
				t.Fatalf("Bills!D%s customer id = %q, want 99", row, got)
			}
		}
	})

	t.Run("blank overrides fall back to the illustrative defaults", func(t *testing.T) {
		workbook, err := buildBillImportTemplate(billImportPurchase, false, billImportTemplateOverrides{})
		if err != nil {
			t.Fatal(err)
		}
		defer workbook.Close()
		if got, _ := workbook.GetCellValue("Bills", "B2"); got != "1" {
			t.Fatalf("Bills!B2 store id = %q, want default 1", got)
		}
	})
}

func TestTemplateOverridesFromQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/bill-import/template?type=sales&store_id=%205%20&branch_id=&customer_id=9", nil)
	got := templateOverridesFromQuery(req)
	want := billImportTemplateOverrides{StoreID: "5", BranchID: "", CustomerID: "9"}
	if got != want {
		t.Fatalf("templateOverridesFromQuery = %+v, want %+v", got, want)
	}
}

func TestBillImportTemplatesHaveStableSheetsLocalizedHeadersAndMultipleBills(t *testing.T) {
	for _, tc := range []struct {
		name       string
		importType string
		arabic     bool
		header     string
	}{
		{"sales english", billImportSales, false, "Product Name"},
		{"purchase arabic", billImportPurchase, true, "اسم المنتج"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workbook, err := buildBillImportTemplate(tc.importType, tc.arabic, billImportTemplateOverrides{})
			if err != nil {
				t.Fatal(err)
			}
			defer workbook.Close()
			if got := workbook.GetSheetList(); len(got) != 2 || got[0] != "Bills" || got[1] != "Products" {
				t.Fatalf("sheet list = %v", got)
			}
			if got, _ := workbook.GetCellValue("Products", "B1"); got != tc.header {
				t.Fatalf("product header = %q, want %q", got, tc.header)
			}
			if got, _ := workbook.GetCellValue("Bills", "A2"); got == "" {
				t.Fatal("first example bill reference is empty")
			}
			if first, _ := workbook.GetCellValue("Bills", "A2"); first == func() string { v, _ := workbook.GetCellValue("Bills", "A3"); return v }() {
				t.Fatal("template must demonstrate multiple bill references")
			}
			headers, _ := workbook.GetRows("Products")
			for _, header := range headers[0] {
				if strings.Contains(strings.ToLower(header), "part number") {
					t.Fatalf("unexpected part-number column: %q", header)
				}
			}
		})
	}
}

func TestParseBillImportWorkbookSupportsArabicAndLineTotalFallback(t *testing.T) {
	workbook := excelize.NewFile()
	_ = workbook.SetSheetName("Sheet1", "Bills")
	_, _ = workbook.NewSheet("Products")
	_ = writeStringRow(workbook, "Bills", 1, []string{"مرجع الفاتورة", "معرف المخزن", "معرف الفرع", "معرف العميل", "اسم العميل", "هاتف العميل", "التاريخ", "الخصم", "ملاحظة", "رقم الهيكل"})
	_ = writeStringRow(workbook, "Bills", 2, []string{"S-1", "1", "1", "", "عميل", "0500", "2026-01-01", "0", "", ""})
	_ = writeStringRow(workbook, "Products", 1, []string{"مرجع الفاتورة", "اسم المنتج", "الكمية", "سعر الوحدة", "إجمالي السطر"})
	_ = writeStringRow(workbook, "Products", 2, []string{"S-1", "فلتر", "2", "", "30"})
	var body bytes.Buffer
	if err := workbook.Write(&body); err != nil {
		t.Fatal(err)
	}
	bills, products, err := parseBillImportWorkbook(body.Bytes(), billImportSales)
	if err != nil {
		t.Fatal(err)
	}
	if len(bills) != 1 || len(products["S-1"]) != 1 {
		t.Fatalf("parsed %d bills and %d products", len(bills), len(products["S-1"]))
	}
	if got := products["S-1"][0].values["unit_price"]; got != "15" {
		t.Fatalf("fallback unit price = %q, want 15", got)
	}
}

func TestHandleImportBillsXLSXCreatesEachPurchaseBill(t *testing.T) {
	helpers.APICache.Flush()
	oldDomain := config.BackendDomain
	t.Cleanup(func() { config.BackendDomain = oldDomain; helpers.APICache.Flush() })
	created := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/purchase_bill" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		created++
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(backend.Close)
	config.BackendDomain = backend.URL
	config.SessionTokensMutex.Lock()
	config.SessionTokens["bulk-import-test"] = "token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "bulk-import-test")
		config.SessionTokensMutex.Unlock()
	})

	workbook, err := buildBillImportTemplate(billImportPurchase, false, billImportTemplateOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	var xlsx bytes.Buffer
	if err := workbook.Write(&xlsx); err != nil {
		t.Fatal(err)
	}
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	_ = writer.WriteField("type", billImportPurchase)
	file, err := writer.CreateFormFile("file", "purchase.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(xlsx.Bytes()); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/bill-import/upload?type=purchase", &form)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.AddCookie(&http.Cookie{Name: "session_id", Value: "bulk-import-test"})
	response := httptest.NewRecorder()
	HandleImportBillsXLSX(response, request)
	if response.Code != http.StatusOK || created != 2 {
		t.Fatalf("status=%d created=%d body=%s", response.Code, created, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"success":2`) {
		t.Fatalf("unexpected result %s", response.Body.String())
	}
}
