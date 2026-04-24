package helpers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestBuildPurchaseBillPayload_ManualProducts verifies that manual-only products
// appear ONLY in manual_products[] and NOT in products[].
func TestBuildPurchaseBillPayload_ManualProducts(t *testing.T) {
	form := url.Values{
		"store_id":    {"1"},
		"supplier_id": {"2"},
		// No catalog products — only manual
		"manual_part_name": {"فلتر زيت", "بواجي"},
		"manual_price":     {"25", "15"},
		"manual_quantity":  {"3", "2"},
		"discount":         {"5"},
		"total_amount":     {"100"},
	}

	req, _ := http.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildPurchaseBillPayload(req)

	// Manual items should be in ManualProducts only
	if len(payload.ManualProducts) != 2 {
		t.Errorf("expected 2 manual_products, got %d", len(payload.ManualProducts))
	}
	for i, p := range payload.ManualProducts {
		if p.Name == "" {
			t.Errorf("manual_product[%d] should have a name set", i)
		}
	}
	// Products must NOT contain duplicates of manual items
	if len(payload.Products) != 0 {
		t.Errorf("products[] should be empty when only manual items exist, got %d items", len(payload.Products))
	}
}

// TestBuildPurchaseBillPayload_CatalogRowWithoutSelection simulates the real
// scenario: user adds a row in the catalog section, types a name, sets price
// and quantity, but never selects from the dropdown so product_id stays 0.
// The item must appear ONLY in manual_products, NOT in products.
func TestBuildPurchaseBillPayload_CatalogRowWithoutSelection(t *testing.T) {
	// This is exactly what the browser sends when user uses the catalog
	// section to add an item without picking from the OEM dropdown
	form := url.Values{
		"store_id":              {"1"},
		"supplier_id":           {"2"},
		"products_product_id":   {"0"},
		"products_part_name":    {"فلتر زيت"},
		"products_price":        {"25"},
		"products_quantity":     {"3"},
		"products_cost_price":   {"20"},
		"products_shelf_number": {"A1"},
		"discount":              {"0"},
		"total_amount":          {"86.25"},
	}

	req, _ := http.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildPurchaseBillPayload(req)

	jsonBytes, _ := json.MarshalIndent(payload, "", "  ")
	t.Logf("Payload JSON:\n%s", string(jsonBytes))

	// Item with product_id=0 must go ONLY to manual_products
	if len(payload.ManualProducts) != 1 {
		t.Errorf("expected 1 manual_product, got %d", len(payload.ManualProducts))
	}

	// products[] must be empty — no duplication
	if len(payload.Products) != 0 {
		t.Errorf("products[] must be empty when product_id=0, got %d items: %+v", len(payload.Products), payload.Products)
	}
}

// TestBuildPurchaseBillPayload_MixedCatalogAndManual simulates: one catalog item
// selected from dropdown (product_id=100) + one manual item from the manual section.
// Each must appear in its own array and nowhere else.
func TestBuildPurchaseBillPayload_MixedCatalogAndManual(t *testing.T) {
	form := url.Values{
		"store_id":             {"1"},
		"supplier_id":          {"2"},
		"products_product_id":  {"100"},
		"products_price":       {"50"},
		"products_quantity":    {"2"},
		"products_part_name":   {"OEM-123"},
		"manual_part_name":     {"فلتر يدوي"},
		"manual_price":         {"30"},
		"manual_quantity":      {"1"},
		"discount":             {"0"},
		"total_amount":         {"130"},
	}

	req, _ := http.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildPurchaseBillPayload(req)

	jsonBytes, _ := json.MarshalIndent(payload, "", "  ")
	t.Logf("Payload JSON:\n%s", string(jsonBytes))

	// Exactly 1 catalog product
	if len(payload.Products) != 1 {
		t.Errorf("expected exactly 1 catalog product, got %d", len(payload.Products))
	}

	// Exactly 1 manual product
	if len(payload.ManualProducts) != 1 {
		t.Errorf("expected exactly 1 manual product, got %d", len(payload.ManualProducts))
	}

	// Verify no data leak between arrays
	if len(payload.Products) > 0 && (payload.Products[0].ProductID == nil || *payload.Products[0].ProductID == 0) {
		t.Error("catalog product must not have ProductID=0 or nil")
	}
	if len(payload.ManualProducts) > 0 {
		mp := payload.ManualProducts[0]
		if mp.Name != "فلتر يدوي" {
			t.Errorf("expected manual name 'فلتر يدوي', got '%s'", mp.Name)
		}
	}
}

// TestBuildPurchaseBillPayload_MixedProducts verifies that when both catalog
// and manual products are provided, they go to separate arrays.
func TestBuildPurchaseBillPayload_MixedProducts(t *testing.T) {
	form := url.Values{
		"store_id":             {"1"},
		"supplier_id":          {"2"},
		"products_product_id":  {"100"},
		"products_price":       {"50"},
		"products_quantity":    {"2"},
		"products_part_name":   {"OEM-123"},
		"manual_part_name":     {"فلتر يدوي"},
		"manual_price":         {"30"},
		"manual_quantity":      {"1"},
		"discount":             {"0"},
		"total_amount":         {"130"},
	}

	req, _ := http.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildPurchaseBillPayload(req)

	// Catalog product should be in Products
	if len(payload.Products) < 1 {
		t.Errorf("expected at least 1 catalog product, got %d", len(payload.Products))
	}

	// Manual product should be in ManualProducts
	if len(payload.ManualProducts) < 1 {
		t.Errorf("expected at least 1 manual product, got %d", len(payload.ManualProducts))
	}

	// Check the manual product data
	if len(payload.ManualProducts) > 0 {
		mp := payload.ManualProducts[0]
		if mp.Name != "فلتر يدوي" {
			t.Errorf("expected manual name 'فلتر يدوي', got '%s'", mp.Name)
		}
		if mp.Price != "30" {
			t.Errorf("expected manual price '30', got '%s'", mp.Price)
		}
		if mp.Quantity != "1" {
			t.Errorf("expected manual quantity '1', got '%s'", mp.Quantity)
		}
	}
}
