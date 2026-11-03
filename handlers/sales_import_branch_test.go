package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// salesBranchBill is a bill of an XLSX sales import, on the given branch. An empty branch is a missing one.
func salesBranchBill(branch string) spreadsheetBill {
	return spreadsheetBill{reference: "SB-1", values: map[string]string{"date": "2026-01-15", "customer_name": "Ahmed", "store_id": "1", "branch_id": branch}}
}

// salesBranchProducts is the one product of the bills of salesBranchBill.
var salesBranchProducts = []spreadsheetProduct{{reference: "SB-1", values: map[string]string{"product_name": "Brake Pad", "quantity": "2", "unit_price": "75"}}}

// TestSalesPayloadNeedsAValidBranch checks that a bill without a valid branch is an error row, not a sale on branch 1.
func TestSalesPayloadNeedsAValidBranch(t *testing.T) {
	for _, branch := range []string{"", "0", "x"} {
		if _, err := salesPayload("batch", 1, salesBranchBill(branch), salesBranchProducts); err == nil {
			t.Errorf("branch %q: a sale without a valid branch was accepted", branch)
		}
	}

	candidates := xlsxSalesCandidates("batch", []spreadsheetBill{salesBranchBill("")}, map[string][]spreadsheetProduct{"SB-1": salesBranchProducts})
	if candidates[0].payload != nil || len(candidates[0].issues) != 1 || candidates[0].issues[0].Code != importCodeInvalidRow {
		t.Fatalf("a bill without a branch = %+v, want one error row with code %s", candidates[0], importCodeInvalidRow)
	}

	payload, err := salesPayload("batch", 1, salesBranchBill("7"), salesBranchProducts)
	if err != nil || payload.BranchID != 7 {
		t.Fatalf("branch 7: payload branch = %d, err = %v, want 7", payload.BranchID, err)
	}
}

// TestCSVSalesImportNeedsABranch checks that a CSV sales import without a branch posts nothing, and that each bill of the
// import posts on the branch that the user chose.
func TestCSVSalesImportNeedsABranch(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	csvBytes := []byte("name,price,quantity\nBrake Pad,75,2\n")
	fields := map[string][]string{"store_id": {"1"}, "selected_rows": {"1"}}

	w := httptest.NewRecorder()
	HandleImportBillsUpload(w, multipartUpload(t, cookie, "/api/invoices/import-csv", "csv_file", "bills.csv", csvBytes, fields))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("csv import without a branch: status = %d body=%s, want 400", w.Code, w.Body.String())
	}
	if n := len(writes.all()); n != 0 {
		t.Fatalf("csv import without a branch posted %d bills", n)
	}

	fields["branch_id"] = []string{"7"}
	posted := runImport(t, writes, func() {
		w := httptest.NewRecorder()
		HandleImportBillsUpload(w, multipartUpload(t, cookie, "/api/invoices/import-csv", "csv_file", "bills.csv", csvBytes, fields))
		if w.Code != http.StatusOK {
			t.Fatalf("csv import on branch 7: status = %d body=%s", w.Code, w.Body.String())
		}
	})
	if len(posted) != 1 || posted[0]["branch_id"] != float64(7) {
		t.Fatalf("csv bills = %v, want one bill on branch 7", posted)
	}
}

// TestDataImportNeedsABranch checks that a sales data import without a branch is refused by its check and its commit.
func TestDataImportNeedsABranch(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	body, err := json.Marshal(map[string]interface{}{
		"type": "bills", "store_id": 1, "mapping": map[string]int{"part_name": 0, "price": 1, "quantity": 2},
		"rows": [][]string{{"Brake Pad", "75", "2"}}, "selected_rows": []int{1},
	})
	if err != nil {
		t.Fatal(err)
	}

	check := httptest.NewRequest(http.MethodPost, "/api/data-import/check", bytes.NewReader(body))
	check.Header.Set("Content-Type", "application/json")
	check.AddCookie(cookie)
	w := httptest.NewRecorder()
	HandleDataImportCheck(w, check)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("data check without a branch: status = %d body=%s, want 400", w.Code, w.Body.String())
	}

	execute := httptest.NewRequest(http.MethodPost, "/api/data-import/execute", bytes.NewReader(body))
	execute.Header.Set("Content-Type", "application/json")
	execute.AddCookie(cookie)
	w = httptest.NewRecorder()
	HandleDataImportExecute(w, execute)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("data commit without a branch: status = %d body=%s, want 400", w.Code, w.Body.String())
	}
	if n := len(writes.all()); n != 0 {
		t.Fatalf("data import without a branch posted %d bills", n)
	}
}

// TestDataImportSendsTheBranch checks that each bill of a sales data import posts on the branch of the request.
func TestDataImportSendsTheBranch(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)
	body, err := json.Marshal(map[string]interface{}{
		"type": "bills", "store_id": 1, "branch_id": 7, "mapping": map[string]int{"part_name": 0, "price": 1, "quantity": 2},
		"rows": [][]string{{"Brake Pad", "75", "2"}}, "selected_rows": []int{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	posted := runImport(t, writes, func() {
		req := httptest.NewRequest(http.MethodPost, "/api/data-import/execute", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		HandleDataImportExecute(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("data import on branch 7: status = %d body=%s", w.Code, w.Body.String())
		}
	})
	if len(posted) != 1 || posted[0]["branch_id"] != float64(7) {
		t.Fatalf("data bills = %v, want one bill on branch 7", posted)
	}
}
