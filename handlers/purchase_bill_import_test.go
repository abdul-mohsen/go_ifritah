package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleDownloadPurchaseBillTemplateReturnsCSVSample(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/purchase-bills/import-template", nil)
	w := httptest.NewRecorder()

	HandleDownloadPurchaseBillTemplate(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "text/csv") {
		t.Fatalf("content type = %q, want csv content type", got)
	}
	if got := resp.Header.Get("Content-Disposition"); !strings.Contains(got, "purchase-bill-template.csv") {
		t.Fatalf("content disposition = %q, want .csv filename", got)
	}
	body := w.Body.String()
	if !strings.Contains(body, "اسم القطعة,الكمية,سعر الشراء,سعر التكلفة,رقم الرف") {
		t.Fatalf("csv should contain import headers, got %q", body)
	}
	if !strings.Contains(body, "مثال,10,100.00,90.00,A1") {
		t.Fatalf("csv should contain example row, got %q", body)
	}
}

func TestHandleDownloadPurchaseBillExcelTemplateReturnsExcelWorkbook(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/purchase-bills/excel-template", nil)
	w := httptest.NewRecorder()

	HandleDownloadPurchaseBillExcelTemplate(w, req)

	workbook, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(len(w.Body.Bytes())))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	worksheet, err := readZipFile(workbook, "xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatalf("read worksheet xml: %v", err)
	}
	if !strings.Contains(string(worksheet), "اسم القطعة") {
		t.Fatalf("worksheet should contain Arabic header, got %s", worksheet)
	}
	if !strings.Contains(string(worksheet), "مثال") {
		t.Fatalf("worksheet should contain example row, got %s", worksheet)
	}
}

func TestHandleParseCSVItemsParsesUploadedCSV(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", "purchase-bill-template.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("اسم القطعة,الكمية,سعر الشراء,سعر التكلفة,رقم الرف\nفلتر زيت,3,25.5,20.25,A1\nبواجي,1,0,0,\n")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/purchase-bills/parse-csv", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	HandleParseCSVItems(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var items []ImportedItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if items[0].PartName != "فلتر زيت" || items[0].Quantity != 3 {
		t.Fatalf("first item = %+v, want parsed csv row", items[0])
	}
	if items[0].PurchasePrice != 25.5 || items[0].CostPrice != 20.25 || items[0].ShelfNumber != "A1" {
		t.Fatalf("first item prices/shelf = %+v, want purchase=25.5 cost=20.25 shelf=A1", items[0])
	}
	if items[1].PartName != "بواجي" || items[1].PurchasePrice != 0 || items[1].CostPrice != 0 {
		t.Fatalf("second item = %+v, want zero-value optional prices preserved", items[1])
	}
}

func TestHandleParseExcelItemsParsesUploadedWorkbook(t *testing.T) {
	workbookBody, err := buildPurchaseBillImportWorkbook([][]string{
		purchaseBillImportTemplateHeader,
		{"فلتر زيت", "3", "25.5", "20.25", "A1"},
		{"بواجي", "1", "0", "0", ""},
	})
	if err != nil {
		t.Fatalf("build workbook: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", "purchase-bill-template.xlsx")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fileWriter.Write(workbookBody); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/purchase-bills/parse-excel", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	HandleParseExcelItems(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var items []ImportedItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if items[0].PartName != "فلتر زيت" || items[0].Quantity != 3 {
		t.Fatalf("first item = %+v, want parsed workbook row", items[0])
	}
	if items[0].PurchasePrice != 25.5 || items[0].CostPrice != 20.25 || items[0].ShelfNumber != "A1" {
		t.Fatalf("first item prices/shelf = %+v, want purchase=25.5 cost=20.25 shelf=A1", items[0])
	}
	if items[1].PartName != "بواجي" || items[1].PurchasePrice != 0 || items[1].CostPrice != 0 {
		t.Fatalf("second item = %+v, want zero-value optional prices preserved", items[1])
	}
}
