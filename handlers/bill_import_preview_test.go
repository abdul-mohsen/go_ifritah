package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"afrita/config"
	"afrita/helpers"

	"github.com/xuri/excelize/v2"
)

// importBackend fakes the bill routes that a sales import calls. It answers each check with answer, records the rows of every
// check and every bill that is posted, and answers a posted bill with the next bill id, from 501.
type importBackend struct {
	mu          sync.Mutex
	answer      func(rows []map[string]interface{}) []map[string]interface{}
	checkStatus int
	checks      [][]map[string]interface{}
	posted      []map[string]interface{}
}

// startImportBackend starts the fake backend, and points the handlers at it for the rest of the test.
func startImportBackend(t *testing.T, answer func(rows []map[string]interface{}) []map[string]interface{}) *importBackend {
	t.Helper()
	backend := &importBackend{answer: answer}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/bill/import/check":
			var body struct {
				Rows []map[string]interface{} `json:"rows"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode check: %v", err)
			}
			backend.mu.Lock()
			backend.checks = append(backend.checks, body.Rows)
			status := backend.checkStatus
			backend.mu.Unlock()
			if status != 0 {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"check refused"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"rows": backend.answer(body.Rows)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/bill":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode bill: %v", err)
			}
			backend.mu.Lock()
			backend.posted = append(backend.posted, body)
			id := 500 + len(backend.posted)
			backend.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(strconv.Itoa(id)))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	previous := config.BackendDomain
	config.BackendDomain = server.URL
	t.Cleanup(func() { config.BackendDomain = previous })
	return backend
}

func (b *importBackend) failChecks(status int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.checkStatus = status
}

func (b *importBackend) checkCalls() [][]map[string]interface{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([][]map[string]interface{}(nil), b.checks...)
}

func (b *importBackend) postedBills() []map[string]interface{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]map[string]interface{}(nil), b.posted...)
}

// allValid answers every row of a check as valid, with no warning.
func allValid(rows []map[string]interface{}) []map[string]interface{} {
	answers := make([]map[string]interface{}, 0, len(rows))
	for i := range rows {
		answers = append(answers, checkAnswer(i, true, nil, nil, nil))
	}
	return answers
}

// checkAnswer is one row of the backend's answer to a check.
func checkAnswer(index int, valid bool, errors, warnings []map[string]interface{}, duplicateOf interface{}) map[string]interface{} {
	if errors == nil {
		errors = []map[string]interface{}{}
	}
	if warnings == nil {
		warnings = []map[string]interface{}{}
	}
	return map[string]interface{}{"index": index, "valid": valid, "errors": errors, "warnings": warnings, "duplicate_of": duplicateOf}
}

// issue is one error or warning of the backend, with its code and message.
func issue(code, message string) map[string]interface{} {
	return map[string]interface{}{"code": code, "message": message}
}

// csvUpload posts a CSV file of sales to a handler, with a session and the rows to post.
func csvUpload(t *testing.T, target, csvBody string, fields map[string][]string) *http.Request {
	t.Helper()
	merged := map[string][]string{"store_id": {"1"}}
	for key, values := range fields {
		merged[key] = values
	}
	return multipartUpload(t, seedSaleSession(t), target, "csv_file", "bills.csv", []byte(csvBody), merged)
}

func decodePreview(t *testing.T, w *httptest.ResponseRecorder) previewResponse {
	t.Helper()
	var preview previewResponse
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatalf("preview body %q: %v", w.Body.String(), err)
	}
	return preview
}

// TestCSVCheckAnswersEachRowWithItsStatus checks that a check keeps the rows in order, the codes and messages of the backend, and
// the duplicate of a row, and that it posts nothing.
func TestCSVCheckAnswersEachRowWithItsStatus(t *testing.T) {
	backend := startImportBackend(t, func(rows []map[string]interface{}) []map[string]interface{} {
		return []map[string]interface{}{
			checkAnswer(0, true, nil, nil, nil),
			checkAnswer(1, true, nil, []map[string]interface{}{issue("duplicate_sale", "this sale matches bill 77 on this branch")}, 77),
			checkAnswer(2, false, []map[string]interface{}{issue("branch_not_found", "branch 9 does not exist"), issue("future_rule", "a rule that this frontend does not know")}, nil, nil),
		}
	})
	w := httptest.NewRecorder()
	HandleCheckBillImportCSV(w, csvUpload(t, "/api/invoices/import-csv/check", "name,price,quantity\nBrake Pad,75,2\nEngine Oil,40,1\nWiper Blade,20,2\n",
		map[string][]string{importSessionField: {importTestSession}}))
	if w.Code != http.StatusOK {
		t.Fatalf("check status = %d body=%s", w.Code, w.Body.String())
	}
	rows := decodePreview(t, w).Rows
	if len(rows) != 3 {
		t.Fatalf("preview has %d rows, want 3", len(rows))
	}
	for i, row := range rows {
		if row.Row != i+1 {
			t.Errorf("row %d is numbered %d", i+1, row.Row)
		}
	}
	if !rows[0].Valid || len(rows[0].Errors) != 0 || len(rows[0].Warnings) != 0 || rows[0].DuplicateOf != nil {
		t.Errorf("row 1 = %+v, want a clean valid row", rows[0])
	}
	if rows[1].DuplicateOf == nil || *rows[1].DuplicateOf != 77 || len(rows[1].Warnings) != 1 || rows[1].Warnings[0].Code != "duplicate_sale" {
		t.Errorf("row 2 = %+v, want a duplicate of bill 77 with its warning", rows[1])
	}
	if rows[2].Valid || len(rows[2].Errors) != 2 || rows[2].Errors[1].Code != "future_rule" || rows[2].Errors[1].Message != "a rule that this frontend does not know" {
		t.Errorf("row 3 = %+v, want both backend errors kept verbatim, and the row blocked", rows[2])
	}
	if calls := backend.checkCalls(); len(calls) != 1 || len(calls[0]) != 3 {
		t.Errorf("checks = %d calls, want one call with 3 rows", len(calls))
	}
	if posted := backend.postedBills(); len(posted) != 0 {
		t.Errorf("a check posted %d bills, want 0", len(posted))
	}
}

// TestCheckSendsRowsInChunksOfTheBackendLimit checks that a check of more than 500 rows is split into calls of at most 500.
func TestCheckSendsRowsInChunksOfTheBackendLimit(t *testing.T) {
	backend := startImportBackend(t, allValid)
	var csvBody strings.Builder
	csvBody.WriteString("name,price,quantity\n")
	for i := 1; i <= 501; i++ {
		fmt.Fprintf(&csvBody, "Part %d,10,1\n", i)
	}
	w := httptest.NewRecorder()
	HandleCheckBillImportCSV(w, csvUpload(t, "/api/invoices/import-csv/check", csvBody.String(), map[string][]string{importSessionField: {importTestSession}}))
	if w.Code != http.StatusOK {
		t.Fatalf("check status = %d body=%s", w.Code, w.Body.String())
	}
	calls := backend.checkCalls()
	if len(calls) != 2 || len(calls[0]) != 500 || len(calls[1]) != 1 {
		t.Fatalf("checks = %d calls, want 500 and 1 rows", len(calls))
	}
	rows := decodePreview(t, w).Rows
	if len(rows) != 501 || rows[500].Row != 501 || !rows[500].Valid {
		t.Fatalf("preview has %d rows, want 501 valid rows in order", len(rows))
	}
}

// TestCheckFailurePostsNothing checks that when the backend cannot check the rows, the answer is an error and no bill is posted.
func TestCheckFailurePostsNothing(t *testing.T) {
	backend := startImportBackend(t, allValid)
	backend.failChecks(http.StatusInternalServerError)
	w := httptest.NewRecorder()
	HandleCheckBillImportCSV(w, csvUpload(t, "/api/invoices/import-csv/check", "name,price\nBrake Pad,75\n", map[string][]string{importSessionField: {importTestSession}}))
	if w.Code != http.StatusBadGateway || !strings.Contains(decodePreview(t, w).Error, "backend check failed") {
		t.Fatalf("status = %d body=%s, want 502 with the check error", w.Code, w.Body.String())
	}
	if posted := backend.postedBills(); len(posted) != 0 {
		t.Errorf("a failed check posted %d bills, want 0", len(posted))
	}
}

// TestCheckBlocksARowTheBackendDoesNotAnswer checks that a row with no answer from the backend is blocked, and is never ok.
func TestCheckBlocksARowTheBackendDoesNotAnswer(t *testing.T) {
	startImportBackend(t, func(rows []map[string]interface{}) []map[string]interface{} {
		return []map[string]interface{}{checkAnswer(0, true, nil, nil, nil)}
	})
	w := httptest.NewRecorder()
	HandleCheckBillImportCSV(w, csvUpload(t, "/api/invoices/import-csv/check", "name,price\nBrake Pad,75\nEngine Oil,40\n", map[string][]string{importSessionField: {importTestSession}}))
	rows := decodePreview(t, w).Rows
	if len(rows) != 2 || rows[1].Valid || len(rows[1].Errors) != 1 || rows[1].Errors[0].Code != importCodeNoAnswer {
		t.Fatalf("row 2 = %+v, want it blocked with the no-answer code", rows)
	}
}

// TestXLSXCheckBlocksABillWithoutProducts checks that a bill with no products is blocked before the backend sees it.
func TestXLSXCheckBlocksABillWithoutProducts(t *testing.T) {
	backend := startImportBackend(t, allValid)
	workbook, err := excelize.OpenReader(bytes.NewReader(importWorkbook(t, billImportSales)))
	if err != nil {
		t.Fatal(err)
	}
	if err := workbook.RemoveRow("Products", 4); err != nil {
		t.Fatal(err)
	}
	var xlsx bytes.Buffer
	if err := workbook.Write(&xlsx); err != nil {
		t.Fatal(err)
	}
	cookie := seedSaleSession(t)
	w := httptest.NewRecorder()
	HandleCheckBillImportXLSX(w, multipartUpload(t, cookie, "/api/bill-import/check?type=sales", "file", "sales.xlsx", xlsx.Bytes(),
		map[string][]string{importSessionField: {importTestSession}}))
	rows := decodePreview(t, w).Rows
	if w.Code != http.StatusOK || len(rows) != 2 {
		t.Fatalf("status = %d rows = %d body=%s", w.Code, len(rows), w.Body.String())
	}
	if rows[1].Valid || len(rows[1].Errors) != 1 || rows[1].Errors[0].Code != importCodeNoProducts {
		t.Errorf("row 2 = %+v, want it blocked with the no-products code", rows[1])
	}
	if calls := backend.checkCalls(); len(calls) != 1 || len(calls[0]) != 1 {
		t.Errorf("checks = %v, want the one row that has products", calls)
	}
}

// TestCommitPostsOnlyTheSelectedRowsWithTheirOwnIdentities checks that a commit posts the selected row, and that the row keeps the
// identity of its number in the workbook.
func TestCommitPostsOnlyTheSelectedRowsWithTheirOwnIdentities(t *testing.T) {
	backend := startImportBackend(t, allValid)
	cookie := seedSaleSession(t)
	w := httptest.NewRecorder()
	HandleImportBillsXLSX(w, multipartUpload(t, cookie, "/api/bill-import/upload?type=sales", "file", "sales.xlsx", importWorkbook(t, billImportSales), saleFields(importTestSession, "2")))
	if w.Code != http.StatusOK {
		t.Fatalf("commit status = %d body=%s", w.Code, w.Body.String())
	}
	posted := backend.postedBills()
	if len(posted) != 1 || posted[0]["bill_request_id"] != "3a3e6962-a4b8-5bfe-9f92-cf2bc2e4a763" {
		t.Fatalf("posted = %v, want one bill with the identity of row 2", posted)
	}
	var result bulkImportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Success != 1 || result.Failed != 0 || len(result.Results) != 1 || result.Results[0].Row != 2 || !result.Results[0].OK || result.Results[0].BillID != 501 {
		t.Errorf("result = %+v, want row 2 created as bill 501", result)
	}
}

// TestCommitDoesNotPostARowTheCheckBlocks checks that a selected row which the check blocks is reported as failed, and that the
// other selected rows are posted.
func TestCommitDoesNotPostARowTheCheckBlocks(t *testing.T) {
	backend := startImportBackend(t, func(rows []map[string]interface{}) []map[string]interface{} {
		return []map[string]interface{}{
			checkAnswer(0, false, []map[string]interface{}{issue("branch_not_found", "branch 9 does not exist")}, nil, nil),
			checkAnswer(1, true, nil, nil, nil),
		}
	})
	w := httptest.NewRecorder()
	HandleImportBillsUpload(w, csvUpload(t, "/api/invoices/import-csv", "name,price,quantity\nBrake Pad,75,2\nEngine Oil,40,1\n", saleFields(importTestSession, "1", "2")))
	if w.Code != http.StatusOK {
		t.Fatalf("commit status = %d body=%s", w.Code, w.Body.String())
	}
	posted := backend.postedBills()
	if len(posted) != 1 || posted[0]["bill_request_id"] != "3a3e6962-a4b8-5bfe-9f92-cf2bc2e4a763" {
		t.Fatalf("posted = %v, want only row 2", posted)
	}
	var result importResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Success != 1 || result.Failed != 1 || result.Results[0].OK || !strings.Contains(result.Results[0].Error, "branch 9 does not exist") || !result.Results[1].OK {
		t.Errorf("result = %+v, want row 1 failed with the backend message and row 2 created", result)
	}
}

// TestCommitRefusesARequestWithoutSelectedRows checks that a sales commit with no selected row is refused, and checks and posts
// nothing.
func TestCommitRefusesARequestWithoutSelectedRows(t *testing.T) {
	backend := startImportBackend(t, allValid)
	w := httptest.NewRecorder()
	HandleImportBillsXLSX(w, multipartUpload(t, seedSaleSession(t), "/api/bill-import/upload?type=sales", "file", "sales.xlsx", importWorkbook(t, billImportSales),
		map[string][]string{importSessionField: {importTestSession}}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", w.Code, w.Body.String())
	}
	if len(backend.checkCalls()) != 0 || len(backend.postedBills()) != 0 {
		t.Errorf("a commit without rows checked or posted something")
	}
}

// TestCommitPostsNothingWhenTheCheckFails checks that a commit whose check fails posts no bill.
func TestCommitPostsNothingWhenTheCheckFails(t *testing.T) {
	backend := startImportBackend(t, allValid)
	backend.failChecks(http.StatusInternalServerError)
	w := httptest.NewRecorder()
	HandleImportBillsXLSX(w, multipartUpload(t, seedSaleSession(t), "/api/bill-import/upload?type=sales", "file", "sales.xlsx", importWorkbook(t, billImportSales), saleFields(importTestSession, "1", "2")))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s, want 502", w.Code, w.Body.String())
	}
	if posted := backend.postedBills(); len(posted) != 0 {
		t.Errorf("a failed check posted %d bills, want 0", len(posted))
	}
}

// TestDataImportCheckAndCommitPostOnlyTheSelectedRows checks the check of a data import, and that its commit posts the selected
// row with the identity of its number.
func TestDataImportCheckAndCommitPostOnlyTheSelectedRows(t *testing.T) {
	backend := startImportBackend(t, allValid)
	cookie := seedSaleSession(t)
	request := importRequest{Type: "bills", StoreID: 1, Mapping: map[string]int{"part_name": 0, "price": 1, "quantity": 2}, Rows: [][]string{{"Brake Pad", "75", "2"}, {"Engine Oil", "40", "1"}}, ImportSession: importTestSession}

	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/data-import/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	HandleDataImportCheck(w, req)
	if rows := decodePreview(t, w).Rows; w.Code != http.StatusOK || len(rows) != 2 || rows[1].Row != 2 || !rows[1].Valid {
		t.Fatalf("check status = %d rows = %+v", w.Code, rows)
	}

	request.SelectedRows = []int{2}
	body, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/data-import/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	HandleDataImportExecute(w, req)
	posted := backend.postedBills()
	if w.Code != http.StatusOK || len(posted) != 1 || posted[0]["bill_request_id"] != "3a3e6962-a4b8-5bfe-9f92-cf2bc2e4a763" {
		t.Fatalf("commit status = %d posted = %v body=%s", w.Code, posted, w.Body.String())
	}
}

// TestXLSXCommitRefusesARowWithAnUnknownCode checks that a row which the backend blocks with a code the frontend does not know is
// not posted, whatever the code is.
func TestXLSXCommitRefusesARowWithAnUnknownCode(t *testing.T) {
	backend := startImportBackend(t, func(rows []map[string]interface{}) []map[string]interface{} {
		return []map[string]interface{}{
			checkAnswer(0, false, []map[string]interface{}{issue("zatca_branch_unregistered", "the branch is not registered with ZATCA")}, nil, nil),
			checkAnswer(1, true, nil, nil, nil),
		}
	})
	w := httptest.NewRecorder()
	HandleImportBillsXLSX(w, multipartUpload(t, seedSaleSession(t), "/api/bill-import/upload?type=sales", "file", "sales.xlsx", importWorkbook(t, billImportSales), saleFields(importTestSession, "1", "2")))
	var result bulkImportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	posted := backend.postedBills()
	if len(posted) != 1 || result.Failed != 1 || result.Results[0].OK || !strings.Contains(result.Results[0].Error, "not registered with ZATCA") {
		t.Fatalf("posted = %d result = %+v, want row 1 refused with its message", len(posted), result)
	}
}

// TestCommitReportsTheDuplicateThatTheCheckFound checks that a posted row says which bill its check named as a duplicate. The page
// compares that bill with the bill that the backend answers with, to tell a replay of an earlier attempt from a new bill.
func TestCommitReportsTheDuplicateThatTheCheckFound(t *testing.T) {
	startImportBackend(t, func(rows []map[string]interface{}) []map[string]interface{} {
		return []map[string]interface{}{checkAnswer(0, true, nil, []map[string]interface{}{issue("duplicate_sale", "this sale matches bill 42 on this branch")}, 42)}
	})
	w := httptest.NewRecorder()
	HandleImportBillsUpload(w, csvUpload(t, "/api/invoices/import-csv", "name,price,quantity\nBrake Pad,75,2\n", saleFields(importTestSession, "1")))
	var result importResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || !result.Results[0].OK || result.Results[0].DuplicateOf == nil || *result.Results[0].DuplicateOf != 42 {
		t.Fatalf("result = %+v, want row 1 posted with duplicate_of 42", result)
	}
}

// TestCheckKeepsAWarningOnlyRowValid checks that a row whose only finding is a warning is valid and has no error, so the preview ticks
// it and the commit may post it.
func TestCheckKeepsAWarningOnlyRowValid(t *testing.T) {
	startImportBackend(t, func(rows []map[string]interface{}) []map[string]interface{} {
		return []map[string]interface{}{
			checkAnswer(0, true, nil, []map[string]interface{}{issue("sale_warning", "this warning does not block the sale")}, nil),
		}
	})
	w := httptest.NewRecorder()
	HandleCheckBillImportCSV(w, csvUpload(t, "/api/invoices/import-csv/check", "name,price,quantity\nBrake Pad,75,2\n", map[string][]string{importSessionField: {importTestSession}}))
	rows := decodePreview(t, w).Rows
	if w.Code != http.StatusOK || len(rows) != 1 {
		t.Fatalf("status = %d rows = %+v, want one row", w.Code, rows)
	}
	if !rows[0].Valid || len(rows[0].Errors) != 0 || len(rows[0].Warnings) != 1 || rows[0].DuplicateOf != nil {
		t.Errorf("row 1 = %+v, want valid with its warning and no error", rows[0])
	}
}

// TestCommitPostsAWarningOnlyRowAndBlocksAnErrorRow checks that a selected row whose only finding is a warning is posted, and that a
// selected row with an error is not posted, even when it has a warning too.
func TestCommitPostsAWarningOnlyRowAndBlocksAnErrorRow(t *testing.T) {
	backend := startImportBackend(t, func(rows []map[string]interface{}) []map[string]interface{} {
		return []map[string]interface{}{
			checkAnswer(0, true, nil, []map[string]interface{}{issue("sale_warning", "this warning does not block the sale")}, nil),
			checkAnswer(1, false, []map[string]interface{}{issue("branch_not_found", "branch 9 does not exist")}, []map[string]interface{}{issue("sale_warning", "this warning does not block the sale")}, nil),
		}
	})
	w := httptest.NewRecorder()
	HandleImportBillsUpload(w, csvUpload(t, "/api/invoices/import-csv", "name,price,quantity\nBrake Pad,75,2\nEngine Oil,40,1\n", saleFields(importTestSession, "1", "2")))
	if w.Code != http.StatusOK {
		t.Fatalf("commit status = %d body=%s", w.Code, w.Body.String())
	}
	posted := backend.postedBills()
	if len(posted) != 1 || posted[0]["bill_request_id"] != "8cb392fa-4ce3-52dd-b421-664a8e39a55a" {
		t.Fatalf("posted = %v, want only row 1 with its identity", posted)
	}
	var result importResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Success != 1 || result.Failed != 1 || !result.Results[0].OK || result.Results[0].BillID != 501 || result.Results[1].OK || !strings.Contains(result.Results[1].Error, "branch 9 does not exist") {
		t.Errorf("result = %+v, want row 1 created as bill 501 and row 2 failed with its error", result)
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

// TestPurchaseImportsAreUnchanged checks that the purchase imports still post their bills, with no session and no preview.
func TestPurchaseImportsAreUnchanged(t *testing.T) {
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
