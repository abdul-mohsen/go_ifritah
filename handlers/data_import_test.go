package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"afrita/config"
	"afrita/helpers"
)

// purchaseMapping reads the test rows in this column order: part name, price, quantity, supplier sequence number,
// cost price, date.
var purchaseMapping = map[string]int{
	"part_name":                0,
	"price":                    1,
	"quantity":                 2,
	"supplier_sequence_number": 3,
	"cost_price":               4,
	"date":                     5,
}

// startPurchaseCapture fakes the purchase bill route and records every purchase bill body that the import posts.
func startPurchaseCapture(t *testing.T) *billWrites {
	t.Helper()
	writes := &billWrites{}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/purchase_bill" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode purchase bill: %v", err)
		}
		writes.add(body)
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
	return writes
}

// runPurchaseImport posts the rows to the data import endpoint as a purchase bill import and returns its result.
func runPurchaseImport(t *testing.T, cookie *http.Cookie, rows [][]string) importResult {
	t.Helper()
	body, err := json.Marshal(importRequest{Type: "purchase_bills", StoreID: 1, Mapping: purchaseMapping, Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/data-import/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	HandleDataImportExecute(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var result importResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v", err)
	}
	return result
}

// manualLines returns the manual_products lines of one posted purchase bill.
func manualLines(t *testing.T, body map[string]interface{}) []map[string]interface{} {
	t.Helper()
	raw, ok := body["manual_products"].([]interface{})
	if !ok {
		t.Fatalf("manual_products = %#v, want a list", body["manual_products"])
	}
	lines := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		line, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("manual line = %#v, want an object", item)
		}
		lines = append(lines, line)
	}
	return lines
}

func TestPurchaseImportSendsSupplierSequenceAndCostPrice(t *testing.T) {
	writes := startPurchaseCapture(t)
	cookie := seedSaleSession(t)

	result := runPurchaseImport(t, cookie, [][]string{{"Brake Pad", "75", "2", "1234", "40", "2026-11-03"}})
	if result.Success != 1 || result.Failed != 0 {
		t.Fatalf("success = %d failed = %d, want 1 and 0 (results %+v)", result.Success, result.Failed, result.Results)
	}
	bodies := writes.all()
	if len(bodies) != 1 {
		t.Fatalf("purchase bills posted = %d, want 1", len(bodies))
	}
	if got := bodies[0]["supplier_sequence_number"]; got != float64(1234) {
		t.Errorf("supplier_sequence_number = %v, want 1234", got)
	}
	lines := manualLines(t, bodies[0])
	if len(lines) != 1 {
		t.Fatalf("manual lines = %d, want 1", len(lines))
	}
	if got := lines[0]["cost_price"]; got != "40" {
		t.Errorf("cost_price = %v, want \"40\"", got)
	}
}

// TestPurchaseImportRejectsRowWithoutSupplierSequenceOrCostPrice sends four invalid rows and one valid row. Only the
// valid row is posted, and each invalid row reports its own error.
func TestPurchaseImportRejectsRowWithoutSupplierSequenceOrCostPrice(t *testing.T) {
	writes := startPurchaseCapture(t)
	cookie := seedSaleSession(t)

	const date = "2026-11-03"
	result := runPurchaseImport(t, cookie, [][]string{
		{"Brake Pad", "75", "2", "", "40", date},
		{"Brake Pad", "75", "2", "abc", "40", date},
		{"Engine Oil", "40", "1", "1236", "", date},
		{"Engine Oil", "40", "1", "1236", "x", date},
		{"Air Filter", "30", "1", "1237", "25", date},
	})
	if result.Success != 1 || result.Failed != 4 || len(result.Results) != 5 {
		t.Fatalf("success = %d failed = %d results = %d, want 1, 4 and 5 (results %+v)", result.Success, result.Failed, len(result.Results), result.Results)
	}
	for i, row := range result.Results {
		if row.OK != (i == 4) {
			t.Errorf("row %d ok = %v, want %v (error %q)", row.Row, row.OK, i == 4, row.Error)
		}
	}
	bodies := writes.all()
	if len(bodies) != 1 {
		t.Fatalf("purchase bills posted = %d, want 1", len(bodies))
	}
	if got := bodies[0]["supplier_sequence_number"]; got != float64(1237) {
		t.Errorf("supplier_sequence_number = %v, want 1237", got)
	}
}

func TestPurchaseImportDateOnlyIsNoonRiyadh(t *testing.T) {
	writes := startPurchaseCapture(t)
	cookie := seedSaleSession(t)

	result := runPurchaseImport(t, cookie, [][]string{
		{"Brake Pad", "75", "2", "1300", "40", "2026-11-03"},
		{"Engine Oil", "40", "1", "1301", "25", "03/11/2026"},
	})
	if result.Success != 2 {
		t.Fatalf("success = %d, want 2 (results %+v)", result.Success, result.Results)
	}
	bodies := writes.all()
	if len(bodies) != 2 {
		t.Fatalf("purchase bills posted = %d, want 2", len(bodies))
	}
	for i, body := range bodies {
		if got := body["effective_date"]; got != "2026-11-03T12:00:00+03:00" {
			t.Errorf("bill %d effective_date = %v, want 2026-11-03T12:00:00+03:00", i+1, got)
		}
	}
}

func TestPurchaseImportRejectsRowWithoutDate(t *testing.T) {
	writes := startPurchaseCapture(t)
	cookie := seedSaleSession(t)

	result := runPurchaseImport(t, cookie, [][]string{
		{"Brake Pad", "75", "2", "1400", "40", ""},
		{"Engine Oil", "40", "1", "1401", "25", "not-a-date"},
	})
	if result.Success != 0 || result.Failed != 2 {
		t.Errorf("success = %d failed = %d, want 0 and 2 (results %+v)", result.Success, result.Failed, result.Results)
	}
	if got := len(writes.all()); got != 0 {
		t.Errorf("purchase bills posted = %d, want 0", got)
	}
}

func TestPurchaseImportSendsManualLinesOnly(t *testing.T) {
	writes := startPurchaseCapture(t)
	cookie := seedSaleSession(t)

	result := runPurchaseImport(t, cookie, [][]string{
		{"Brake Pad", "75", "2", "1500", "40", "2026-11-03"},
		{"Engine Oil", "40", "1", "1501", "25", "2026-11-03"},
	})
	if result.Success != 2 {
		t.Fatalf("success = %d, want 2 (results %+v)", result.Success, result.Results)
	}
	bodies := writes.all()
	if len(bodies) != 2 {
		t.Fatalf("purchase bills posted = %d, want 2", len(bodies))
	}
	for i, body := range bodies {
		if products, ok := body["products"].([]interface{}); !ok || len(products) != 0 {
			t.Errorf("bill %d products = %#v, want an empty list", i+1, body["products"])
		}
		if lines := manualLines(t, body); len(lines) != 1 {
			t.Errorf("bill %d manual lines = %d, want 1", i+1, len(lines))
		}
	}
}
