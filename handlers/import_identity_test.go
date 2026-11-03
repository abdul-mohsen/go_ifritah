package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var uuidV5Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// importTestSession and importTestNextSession are two UUID v4 sessions, one per sales import.
const (
	importTestSession     = "0e8b3c2a-5d7f-4a1b-9c6e-3f2d1a0b9c8e"
	importTestNextSession = "5c1f9a7e-2b3d-4e8f-8a6c-1d0e9f8b7a65"
)

// The golden values come from an independent UUID v5 implementation (Node crypto and Python uuid5), over the names
// "<session>:1" and "<session>:2" under the import namespace.
func TestImportSaleIdentityGoldenValue(t *testing.T) {
	posID, requestID := importSaleIdentity(importTestSession, 1)
	if posID != "import" || requestID != "8cb392fa-4ce3-52dd-b421-664a8e39a55a" {
		t.Fatalf("row 1 identity = %q %q, want import 8cb392fa-4ce3-52dd-b421-664a8e39a55a", posID, requestID)
	}
	_, second := importSaleIdentity(importTestSession, 2)
	if second != "3a3e6962-a4b8-5bfe-9f92-cf2bc2e4a763" {
		t.Fatalf("row 2 bill_request_id = %q, want 3a3e6962-a4b8-5bfe-9f92-cf2bc2e4a763", second)
	}
}

func TestImportSaleIdentityIsStableForASessionAndRow(t *testing.T) {
	_, first := importSaleIdentity(importTestSession, 1)
	_, again := importSaleIdentity(importTestSession, 1)
	_, nextRow := importSaleIdentity(importTestSession, 2)
	_, otherSession := importSaleIdentity(importTestNextSession, 1)
	if first != again {
		t.Errorf("same session and row gave %s and %s", first, again)
	}
	if first == nextRow {
		t.Errorf("row 1 and row 2 of one session share the identity %s", first)
	}
	if first == otherSession {
		t.Errorf("a new session shares the identity %s of row 1", first)
	}
	if !uuidV5Pattern.MatchString(first) {
		t.Errorf("bill_request_id %q is not a UUID v5", first)
	}
}

func TestValidImportSession(t *testing.T) {
	cases := map[string]bool{
		importTestSession:                      true,
		"":                                     false,
		"0E8B3C2A-5D7F-4A1B-9C6E-3F2D1A0B9C8E": false,
		"0e8b3c2a-5d7f-5a1b-9c6e-3f2d1a0b9c8e": false,
		"0e8b3c2a-5d7f-4a1b-1c6e-3f2d1a0b9c8e": false,
		"not-a-session":                        false,
	}
	for session, want := range cases {
		if got := validImportSession(session); got != want {
			t.Errorf("validImportSession(%q) = %v, want %v", session, got, want)
		}
	}
}

func TestXLSXSalesImportSendsSaleIdentity(t *testing.T) {
	enableSalesImport(t)
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	xlsx := importWorkbook(t, billImportSales)
	upload := func(session string) []map[string]interface{} {
		return runImport(t, writes, func() {
			req := multipartUpload(t, cookie, "/api/bill-import/upload?type=sales", "file", "sales.xlsx", xlsx, map[string]string{importSessionField: session})
			w := httptest.NewRecorder()
			HandleImportBillsXLSX(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("xlsx import status = %d body=%s", w.Code, w.Body.String())
			}
		})
	}

	first := upload(importTestSession)
	assertSameImportIdentities(t, first, upload(importTestSession))
	assertNewImportIdentities(t, first, upload(importTestNextSession))
}

func TestCSVImportSendsSaleIdentity(t *testing.T) {
	enableSalesImport(t)
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	csvBytes := []byte("name,price,quantity\nBrake Pad,75,2\nEngine Oil,40,1\n")
	upload := func(session string) []map[string]interface{} {
		return runImport(t, writes, func() {
			req := multipartUpload(t, cookie, "/api/invoices/import-csv", "csv_file", "bills.csv", csvBytes, map[string]string{"store_id": "1", importSessionField: session})
			w := httptest.NewRecorder()
			HandleImportBillsUpload(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("csv import status = %d body=%s", w.Code, w.Body.String())
			}
		})
	}

	first := upload(importTestSession)
	assertSameImportIdentities(t, first, upload(importTestSession))
	assertNewImportIdentities(t, first, upload(importTestNextSession))
}

func TestDataImportSendsSaleIdentity(t *testing.T) {
	enableSalesImport(t)
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	mapping := map[string]int{"part_name": 0, "price": 1, "quantity": 2}
	rows := [][]string{{"Brake Pad", "75", "2"}, {"Engine Oil", "40", "1"}}
	execute := func(session string) []map[string]interface{} {
		body, err := json.Marshal(importRequest{Type: "bills", StoreID: 1, Mapping: mapping, Rows: rows, ImportSession: session})
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

	first := execute(importTestSession)
	assertSameImportIdentities(t, first, execute(importTestSession))
	assertNewImportIdentities(t, first, execute(importTestNextSession))
}

// TestSalesImportRefusesAMissingSessionWhileItIsOn checks that a sales import without a session that is a UUID v4
// answers 400 and posts no bill.
func TestSalesImportRefusesAMissingSessionWhileItIsOn(t *testing.T) {
	enableSalesImport(t)
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
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
			body, err := json.Marshal(importRequest{Type: "bills", StoreID: 1, Mapping: map[string]int{"part_name": 0, "price": 1}, Rows: [][]string{{"Brake Pad", "75"}}})
			if err != nil {
				t.Fatal(err)
			}
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
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), importSessionMessage) {
				t.Fatalf("status = %d body=%s, want 400 with the session message", w.Code, w.Body.String())
			}
		})
	}
	if got := len(writes.all()); got != 0 {
		t.Fatalf("imports without a session posted %d bills, want 0", got)
	}
}

// multipartUpload builds a signed-in request that posts one file, with optional form fields.
func multipartUpload(t *testing.T, cookie *http.Cookie, target, field, name string, content []byte, fields map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
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

// importWorkbook returns the import template of one bill type, which holds two bills.
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

// assertSameImportIdentities checks that a retry in the same session sends the same bodies, and that the bills of one
// import do not share an identity.
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
			t.Errorf("bill %d: retry body differs from the first attempt: %v != %v", i+1, again[i], first[i])
		}
		if seen[first[i]["bill_request_id"]] {
			t.Errorf("bill %d reuses the bill_request_id %v of an earlier bill", i+1, first[i]["bill_request_id"])
		}
		seen[first[i]["bill_request_id"]] = true
	}
}

// assertNewImportIdentities checks that an import in a new session sends a new identity for every row.
func assertNewImportIdentities(t *testing.T, first, next []map[string]interface{}) {
	t.Helper()
	if len(next) != len(first) {
		t.Fatalf("import in a new session sent %d bills, want %d", len(next), len(first))
	}
	for i := range first {
		assertImportIdentity(t, next[i])
		if next[i]["bill_request_id"] == first[i]["bill_request_id"] {
			t.Errorf("bill %d kept the bill_request_id %v in a new session", i+1, first[i]["bill_request_id"])
		}
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
