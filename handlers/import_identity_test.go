package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"testing"
)

var uuidV5Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// The golden values come from an independent UUID v5 implementation (Node crypto and Python uuid5), over
// sha256("example") and the names "<hex>:1" and "<hex>:2".
func TestImportSaleIdentityGoldenValue(t *testing.T) {
	batch := importBatchKey([]byte("example"))
	posID, requestID := importSaleIdentity(batch, 1)
	if posID != "import" || requestID != "4311c7b2-7e14-5ad9-8c56-463a42bfd552" {
		t.Fatalf("row 1 identity = %q %q, want import 4311c7b2-7e14-5ad9-8c56-463a42bfd552", posID, requestID)
	}
	_, second := importSaleIdentity(batch, 2)
	if second != "338b33a2-e22a-52d0-be14-1a6b2cbfcc71" {
		t.Fatalf("row 2 bill_request_id = %q, want 338b33a2-e22a-52d0-be14-1a6b2cbfcc71", second)
	}
}

func TestImportSaleIdentityIsStableForABatchAndRow(t *testing.T) {
	batch := importBatchKey([]byte("file"))
	_, first := importSaleIdentity(batch, 1)
	_, again := importSaleIdentity(batch, 1)
	_, nextRow := importSaleIdentity(batch, 2)
	_, otherFile := importSaleIdentity(importBatchKey([]byte("file, edited")), 1)
	if first != again {
		t.Errorf("same batch and row gave %s and %s", first, again)
	}
	if first == nextRow {
		t.Errorf("row 1 and row 2 of one batch share the identity %s", first)
	}
	if first == otherFile {
		t.Errorf("a different file shares the identity %s of row 1", first)
	}
	if !uuidV5Pattern.MatchString(first) {
		t.Errorf("bill_request_id %q is not a UUID v5", first)
	}
}

func TestXLSXSalesImportSendsSaleIdentity(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	xlsx := importWorkbook(t, billImportSales)
	upload := func() []map[string]interface{} {
		return runImport(t, writes, func() {
			req := multipartUpload(t, cookie, "/api/bill-import/upload?type=sales", "file", "sales.xlsx", xlsx, saleFields("1", "2"))
			w := httptest.NewRecorder()
			HandleImportBillsXLSX(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("xlsx import status = %d body=%s", w.Code, w.Body.String())
			}
		})
	}

	first := upload()
	assertSameImportIdentities(t, first, upload())
}

func TestCSVImportSendsSaleIdentity(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	csvBytes := []byte("name,price,quantity\nBrake Pad,75,2\nEngine Oil,40,1\n")
	upload := func(content []byte) []map[string]interface{} {
		return runImport(t, writes, func() {
			fields := saleFields("1", "2")
			fields["store_id"] = []string{"1"}
			fields["branch_id"] = []string{"7"}
			req := multipartUpload(t, cookie, "/api/invoices/import-csv", "csv_file", "bills.csv", content, fields)
			w := httptest.NewRecorder()
			HandleImportBillsUpload(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("csv import status = %d body=%s", w.Code, w.Body.String())
			}
		})
	}

	first := upload(csvBytes)
	assertSameImportIdentities(t, first, upload(csvBytes))

	edited := upload(append(append([]byte{}, csvBytes...), []byte("Wiper Blade,20,2\n")...))
	if edited[0]["bill_request_id"] == first[0]["bill_request_id"] {
		t.Errorf("an edited file reused the identity %v of row 1", first[0]["bill_request_id"])
	}
}

func TestDataImportSendsSaleIdentity(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	mapping := map[string]int{"part_name": 0, "price": 1, "quantity": 2}
	rows := [][]string{{"Brake Pad", "75", "2"}, {"Engine Oil", "40", "1"}}
	execute := func(rows [][]string) []map[string]interface{} {
		body, err := json.Marshal(importRequest{Type: "bills", StoreID: 1, BranchID: 7, Mapping: mapping, Rows: rows, SelectedRows: []int{1, 2}})
		if err != nil {
			t.Fatal(err)
		}
		return runImport(t, writes, func() {
			req := httptest.NewRequest(http.MethodPost, "/api/data-import/execute", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			HandleDataImportExecute(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("data import status = %d body=%s", w.Code, w.Body.String())
			}
		})
	}

	first := execute(rows)
	assertSameImportIdentities(t, first, execute(rows))

	edited := execute(append(append([][]string{}, rows...), []string{"Wiper Blade", "20", "2"}))
	if edited[0]["bill_request_id"] == first[0]["bill_request_id"] {
		t.Errorf("an edited import reused the identity %v of row 1", first[0]["bill_request_id"])
	}
}

// saleFields are the form fields of a sales commit: the rows that it posts.
func saleFields(rows ...string) map[string][]string {
	return map[string][]string{"selected_rows": rows}
}

// multipartUpload builds a signed-in request that posts one file, with optional form fields.
func multipartUpload(t *testing.T, cookie *http.Cookie, target, field, name string, content []byte, fields map[string][]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, values := range fields {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)
	return req
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

// runImport runs an import and returns the bill bodies that the backend received during it.
func runImport(t *testing.T, writes *billWrites, run func()) []map[string]interface{} {
	t.Helper()
	before := len(writes.all())
	run()
	return writes.all()[before:]
}

// assertSameImportIdentities checks that a re-import of the same content sends the same bodies, and that the bills of one
// batch do not share an identity.
func assertSameImportIdentities(t *testing.T, first, again []map[string]interface{}) {
	t.Helper()
	if len(first) < 2 {
		t.Fatalf("import sent %d bills, want at least 2 to compare rows", len(first))
	}
	if len(again) != len(first) {
		t.Fatalf("re-import sent %d bills, want %d", len(again), len(first))
	}
	seen := map[interface{}]bool{}
	for i := range first {
		assertImportIdentity(t, first[i])
		if !reflect.DeepEqual(first[i], again[i]) {
			t.Errorf("bill %d: re-import body differs from the first import: %v != %v", i+1, again[i], first[i])
		}
		if seen[first[i]["bill_request_id"]] {
			t.Errorf("bill %d reuses the bill_request_id %v of an earlier bill", i+1, first[i]["bill_request_id"])
		}
		seen[first[i]["bill_request_id"]] = true
	}
}

func assertImportIdentity(t *testing.T, body map[string]interface{}) {
	t.Helper()
	if body["pos_id"] != "import" {
		t.Errorf("pos_id = %v, want import", body["pos_id"])
	}
	requestID, _ := body["bill_request_id"].(string)
	if !uuidV5Pattern.MatchString(requestID) {
		t.Errorf("bill_request_id = %q, want a UUID v5", requestID)
	}
}
