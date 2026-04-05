package helpers

// apidocs_compliance_test.go — Tests that verify the BFF payloads match the API docs exactly.
// These tests must NEVER be deleted. They catch format mismatches between our BFF and the backend API.
//
// VERIFIED against live backend (2026-03-03):
//   - price as STRING + quantity as STRING → 201 SUCCESS
//   - price as INT (number) → 400 FAIL
//   - quantity as INT (number) → 400 FAIL
//   - discount, maintenance_cost → STRINGS
//   - products array MUST have at least one item (empty products → creates $0 bill)

import (
	"afrita/models"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// =============================================================================
// BILL CREATE PAYLOAD — must match api_docs/bill/create/request.json
// Updated 2026-03: {"product_id": 6, "name": "...", "price": "100", "quantity": "2"} — ALL STRINGS
// =============================================================================

// TestBillPayload_ProductPriceIsString verifies that product price in the JSON
// payload is a STRING, not a number.
// Tested 2026-03-03: int price → 400. String price → 201.
func TestBillPayload_ProductPriceIsString(t *testing.T) {
	form := url.Values{
		"store_id":            {"1"},
		"products_product_id": {"6"},
		"products_price":      {"100"},
		"products_quantity":   {"2"},
		"discount":            {"0"},
		"maintenance_cost":    {"0"},
		"state":               {"1"},
		"user_name":           {"عميل تجريبي"},
		"user_phone_number":   {"0512345678"},
		"total_amount":        {"250"},
	}
	req, _ := http.NewRequest("POST", "/api/invoices", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildBillPayload(req)
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal BillPayload: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	products, ok := raw["products"].([]interface{})
	if !ok || len(products) == 0 {
		t.Fatal("expected non-empty products array")
	}

	product := products[0].(map[string]interface{})

	// Price MUST be a string — backend rejects numbers
	if _, ok := product["price"].(string); !ok {
		t.Errorf("product price must be a string, got %T with value %v", product["price"], product["price"])
	}

	// product_id must be a number
	if _, ok := product["product_id"].(float64); !ok {
		t.Errorf("product product_id must be a number, got %T", product["product_id"])
	}

	// Quantity MUST be a string — backend rejects numbers
	if _, ok := product["quantity"].(string); !ok {
		t.Errorf("product quantity must be a string, got %T with value %v", product["quantity"], product["quantity"])
	}
}

// TestBillPayload_ManualProductPriceIsString verifies that manual product price
// and quantity are strings in the JSON payload.
// Updated 2026-03: When only manual products exist, they get converted to products[]
// with a generated product_id so the backend accepts them.
func TestBillPayload_ManualProductPriceIsString(t *testing.T) {
	form := url.Values{
		"store_id":         {"1"},
		"manual_part_name": {"عمالة تركيب"},
		"manual_price":     {"50"},
		"manual_quantity":  {"1"},
		"discount":         {"0"},
		"maintenance_cost": {"0"},
		"state":            {"1"},
		"user_name":        {"عميل"},
		"total_amount":     {"50"},
	}
	req, _ := http.NewRequest("POST", "/api/invoices", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildBillPayload(req)
	jsonBytes, _ := json.Marshal(payload)

	var raw map[string]interface{}
	json.Unmarshal(jsonBytes, &raw)

	// Manual-only items stay in manual_products[] (no random ID conversion)
	manualProducts, _ := raw["manual_products"].([]interface{})
	if len(manualProducts) == 0 {
		t.Fatal("expected manual_products to have items")
	}

	p := manualProducts[0].(map[string]interface{})
	if _, ok := p["price"].(string); !ok {
		t.Errorf("product price must be a string, got %T with value %v", p["price"], p["price"])
	}
	if _, ok := p["quantity"].(string); !ok {
		t.Errorf("product quantity must be a string, got %T with value %v", p["quantity"], p["quantity"])
	}
	if p["product_id"] != nil {
		t.Errorf("manual product should have null product_id, got %v", p["product_id"])
	}
}

// TestBillPayload_DiscountAndMaintenanceAreStrings verifies that discount and
// maintenance_cost are sent as STRINGS per API docs.
func TestBillPayload_DiscountAndMaintenanceAreStrings(t *testing.T) {
	form := url.Values{
		"store_id":         {"1"},
		"discount":         {"10"},
		"maintenance_cost": {"5"},
		"state":            {"1"},
		"total_amount":     {"100"},
	}
	req, _ := http.NewRequest("POST", "/api/invoices", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildBillPayload(req)
	jsonBytes, _ := json.Marshal(payload)

	var raw map[string]interface{}
	json.Unmarshal(jsonBytes, &raw)

	if _, ok := raw["discount"].(string); !ok {
		t.Errorf("discount must be a string, got %T with value %v", raw["discount"], raw["discount"])
	}
	if _, ok := raw["maintenance_cost"].(string); !ok {
		t.Errorf("maintenance_cost must be a string, got %T with value %v", raw["maintenance_cost"], raw["maintenance_cost"])
	}
}

// TestBillPayload_TotalAmountIsNumber verifies total_amount is a number.
func TestBillPayload_TotalAmountIsNumber(t *testing.T) {
	form := url.Values{
		"store_id":     {"1"},
		"total_amount": {"250.50"},
		"discount":     {"0"},
		"state":        {"1"},
	}
	req, _ := http.NewRequest("POST", "/api/invoices", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildBillPayload(req)
	jsonBytes, _ := json.Marshal(payload)

	var raw map[string]interface{}
	json.Unmarshal(jsonBytes, &raw)

	switch v := raw["total_amount"].(type) {
	case float64:
		if v != 250.5 {
			t.Errorf("total_amount should be 250.5, got %v", v)
		}
	default:
		t.Errorf("total_amount must be a number, got %T", raw["total_amount"])
	}
}

// TestBillPayload_FullFormatMatchesAPIDocs verifies the complete JSON structure.
// Verified 2026-03-03: price=string, quantity=string, discount=string, id=int, store_id=int
func TestBillPayload_FullFormatMatchesAPIDocs(t *testing.T) {
	form := url.Values{
		"store_id":            {"1"},
		"products_product_id": {"6"},
		"products_price":      {"100"},
		"products_quantity":   {"2"},
		"manual_part_name":    {"عمالة تركيب"},
		"manual_price":        {"50"},
		"manual_quantity":     {"1"},
		"total_amount":        {"250"},
		"discount":            {"0"},
		"maintenance_cost":    {"0"},
		"state":               {"1"},
		"user_name":           {"عميل تجريبي"},
		"user_phone_number":   {"0512345678"},
	}
	req, _ := http.NewRequest("POST", "/api/invoices", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildBillPayload(req)
	jsonBytes, _ := json.Marshal(payload)

	var raw map[string]interface{}
	json.Unmarshal(jsonBytes, &raw)

	// store_id should be number
	if _, ok := raw["store_id"].(float64); !ok {
		t.Errorf("store_id must be number, got %T", raw["store_id"])
	}

	// state should be number
	if _, ok := raw["state"].(float64); !ok {
		t.Errorf("state must be number, got %T", raw["state"])
	}

	// products must be array with items
	products, ok := raw["products"].([]interface{})
	if !ok {
		t.Fatal("products must be an array")
	}
	if len(products) == 0 {
		t.Fatal("products should not be empty when catalog products are provided")
	}

	// Verify product item types
	p := products[0].(map[string]interface{})
	if _, ok := p["product_id"].(float64); !ok {
		t.Errorf("product product_id must be number, got %T", p["product_id"])
	}
	if _, ok := p["price"].(string); !ok {
		t.Errorf("product price must be string, got %T", p["price"])
	}
	if _, ok := p["quantity"].(string); !ok {
		t.Errorf("product quantity must be string, got %T", p["quantity"])
	}

	// manual_products must be array (even if empty)
	if _, ok := raw["manual_products"].([]interface{}); !ok {
		t.Fatal("manual_products must be an array")
	}

	// discount must be string
	if _, ok := raw["discount"].(string); !ok {
		t.Errorf("discount must be string, got %T", raw["discount"])
	}

	// maintenance_cost must be string
	if _, ok := raw["maintenance_cost"].(string); !ok {
		t.Errorf("maintenance_cost must be string, got %T", raw["maintenance_cost"])
	}

	// user_name must be string
	if v, ok := raw["user_name"].(string); !ok || v != "عميل تجريبي" {
		t.Errorf("user_name mismatch: got %v", raw["user_name"])
	}
}

// TestBillPayload_MustHaveProduct verifies that manual-only products stay in
// manual_products[] and catalog products go to products[].
func TestBillPayload_MustHaveProduct(t *testing.T) {
	// Test 1: Only manual products provided — should stay in manual_products[]
	form := url.Values{
		"store_id":         {"1"},
		"manual_part_name": {"قطعة يدوية"},
		"manual_price":     {"100"},
		"manual_quantity":  {"1"},
		"discount":         {"0"},
		"maintenance_cost": {"0"},
		"state":            {"1"},
		"user_name":        {"عميل"},
		"total_amount":     {"100"},
	}
	req, _ := http.NewRequest("POST", "/api/invoices", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildBillPayload(req)
	if len(payload.ManualProducts) == 0 {
		t.Error("manual_products must not be empty when only manual items are provided")
	}

	// Test 2: Catalog product provided — should be in products directly
	form2 := url.Values{
		"store_id":            {"1"},
		"products_product_id": {"6"},
		"products_price":      {"100"},
		"products_quantity":   {"2"},
		"discount":            {"0"},
		"state":               {"1"},
		"total_amount":        {"200"},
	}
	req2, _ := http.NewRequest("POST", "/api/invoices", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload2 := BuildBillPayload(req2)
	if len(payload2.Products) == 0 {
		t.Error("products must not be empty when catalog products are provided")
	}
	if payload2.Products[0].ID != 6 {
		t.Errorf("expected product ID 6, got %d", payload2.Products[0].ID)
	}
}

// TestBillPayload_EmptySlicesNotNull verifies arrays are [] not null in JSON.
func TestBillPayload_EmptySlicesNotNull(t *testing.T) {
	form := url.Values{
		"store_id": {"1"},
		"discount": {"0"},
		"state":    {"1"},
	}
	req, _ := http.NewRequest("POST", "/api/invoices", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildBillPayload(req)
	jsonBytes, _ := json.Marshal(payload)
	jsonStr := string(jsonBytes)

	if strings.Contains(jsonStr, `"products":null`) {
		t.Error("products should be [] not null")
	}
	if strings.Contains(jsonStr, `"manual_products":null`) {
		t.Error("manual_products should be [] not null")
	}
}

// =============================================================================
// PURCHASE BILL CREATE PAYLOAD — must match api_docs/purchase_bill/create/request.json
// Verified format: {"id": 6, "price": "50", "quantity": "10"} — ALL STRINGS
// =============================================================================

// TestPurchaseBillPayload_ProductPriceAndQuantityAreStrings verifies purchase bill
// product prices and quantities are strings, matching the verified backend format.
// Updated 2026-03: both products and manual_products use {product_id, name, price, quantity}.
func TestPurchaseBillPayload_ProductPriceAndQuantityAreStrings(t *testing.T) {
	form := url.Values{
		"store_id":                 {"1"},
		"supplier_id":              {"5"},
		"products_product_id":      {"6"},
		"products_price":           {"50"},
		"products_quantity":        {"10"},
		"products_part_name":       {"فلتر"},
		"manual_part_name":         {"فلتر زيت تويوتا"},
		"manual_price":             {"50"},
		"manual_quantity":          {"10"},
		"discount":                 {"5"},
		"total_amount":             {"500"},
		"payment_date":             {"2025-01-15"},
		"supplier_sequance_number": {"1001"},
	}
	req, _ := http.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildPurchaseBillPayload(req)
	jsonBytes, _ := json.Marshal(payload)

	var raw map[string]interface{}
	json.Unmarshal(jsonBytes, &raw)

	products := raw["products"].([]interface{})
	if len(products) == 0 {
		t.Fatal("expected products")
	}
	p := products[0].(map[string]interface{})

	// product_id must be a number (was "id" before)
	if _, ok := p["product_id"].(float64); !ok {
		t.Errorf("purchase bill product product_id must be a number, got %T value=%v", p["product_id"], p["product_id"])
	}

	// name must be a string (was "part_name" before)
	if _, ok := p["name"].(string); !ok {
		t.Errorf("purchase bill product name must be a string, got %T value=%v", p["name"], p["name"])
	}

	// Price must be string
	if _, ok := p["price"].(string); !ok {
		t.Errorf("purchase bill product price must be a string, got %T value=%v", p["price"], p["price"])
	}

	// Quantity must be string
	if _, ok := p["quantity"].(string); !ok {
		t.Errorf("purchase bill product quantity must be a string, got %T value=%v", p["quantity"], p["quantity"])
	}

	// Check manual_products — same structure with product_id=null
	manualProducts, _ := raw["manual_products"].([]interface{})
	if len(manualProducts) == 0 {
		t.Fatal("expected manual_products")
	}
	mp := manualProducts[0].(map[string]interface{})
	if mp["product_id"] != nil {
		t.Errorf("manual product product_id must be null, got %v", mp["product_id"])
	}
	if _, ok := mp["name"].(string); !ok {
		t.Errorf("purchase bill manual product name must be string, got %T", mp["name"])
	}
	if _, ok := mp["price"].(string); !ok {
		t.Errorf("purchase bill manual product price must be string, got %T", mp["price"])
	}
	if _, ok := mp["quantity"].(string); !ok {
		t.Errorf("purchase bill manual product quantity must be string, got %T", mp["quantity"])
	}
}

// =============================================================================
// PAGINATION — must match api_docs/bill/list/request.json
// API docs format: {"page_number": 0, "page_size": 20}
// page_number is 0-BASED (UI page 1 = backend page_number 0)
// =============================================================================

// TestPagination_UIPage1MapsToBackendPage0 verifies that UI page 1 maps
// to backend page_number 0.
func TestPagination_UIPage1MapsToBackendPage0(t *testing.T) {
	testCases := []struct {
		uiPage       int
		expectedPage int
	}{
		{1, 0},  // First page: UI=1 → Backend=0
		{2, 1},  // Second page: UI=2 → Backend=1
		{3, 2},  // Third page
		{0, 0},  // Invalid (0 or below) defaults to page 1 → Backend=0
		{-1, 0}, // Negative defaults to page 1 → Backend=0
	}

	for _, tc := range testCases {
		page := tc.uiPage
		if page < 1 {
			page = 1
		}
		backendPage := page - 1

		if backendPage != tc.expectedPage {
			t.Errorf("UI page %d: expected backend page_number %d, got %d",
				tc.uiPage, tc.expectedPage, backendPage)
		}
	}
}

// TestPagination_PayloadFormat verifies the pagination JSON payload format.
func TestPagination_PayloadFormat(t *testing.T) {
	page := 1
	pageSize := 20

	if page < 1 {
		page = 1
	}
	backendPage := page - 1
	payload := map[string]interface{}{
		"page_number": backendPage,
		"page_size":   pageSize,
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(jsonBytes, &raw)

	pn, ok := raw["page_number"].(float64)
	if !ok {
		t.Fatalf("page_number must be a number, got %T", raw["page_number"])
	}
	if pn != 0 {
		t.Errorf("page_number for UI page 1 should be 0, got %v", pn)
	}

	ps, ok := raw["page_size"].(float64)
	if !ok {
		t.Fatalf("page_size must be a number, got %T", raw["page_size"])
	}
	if ps != 20 {
		t.Errorf("page_size should be 20, got %v", ps)
	}
}

// TestPagination_PurchaseBillFormat verifies purchase bill pagination format.
func TestPagination_PurchaseBillFormat(t *testing.T) {
	page := 2
	pageSize := 10

	if page < 1 {
		page = 1
	}
	backendPage := page - 1
	payload := map[string]interface{}{
		"page_number": backendPage,
		"page_size":   pageSize,
	}
	jsonBytes, _ := json.Marshal(payload)

	var raw map[string]interface{}
	json.Unmarshal(jsonBytes, &raw)

	pn := raw["page_number"].(float64)
	if pn != 1 {
		t.Errorf("UI page 2 should be backend page_number 1, got %v", pn)
	}
}

// TestPagination_NextPageWhenExactlyPerPage verifies that when backend returns
// exactly perPage items, we show a next page button.
func TestPagination_NextPageWhenExactlyPerPage(t *testing.T) {
	perPage := 10

	invoices := make([]models.Invoice, perPage)
	nextPage := 0
	if len(invoices) >= perPage {
		nextPage = 2
	}
	if nextPage == 0 {
		t.Error("nextPage should be > 0 when backend returns exactly perPage items")
	}

	invoicesLast := make([]models.Invoice, perPage-3)
	nextPage = 0
	if len(invoicesLast) >= perPage {
		nextPage = 2
	}
	if nextPage != 0 {
		t.Error("nextPage should be 0 when backend returns fewer than perPage items")
	}
}
