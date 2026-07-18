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
		if p.PartName == "" {
			t.Errorf("manual_product[%d] should have a part_name set", i)
		}
	}
	// Products must NOT contain duplicates of manual items
	if len(payload.Products) != 0 {
		t.Errorf("products[] should be empty when only manual items exist, got %d items", len(payload.Products))
	}
}

// TestBuildPurchaseBillPayload_UnselectedItemIsManual verifies that a typed
// item becomes a manual line unless the user chooses an inventory result.
func TestBuildPurchaseBillPayload_UnselectedItemIsManual(t *testing.T) {
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

	if len(payload.Products) != 0 {
		t.Errorf("expected no stock products, got %d", len(payload.Products))
	}
	if len(payload.ManualProducts) != 1 {
		t.Errorf("expected 1 manual product, got %d", len(payload.ManualProducts))
	}
	if got := payload.ManualProducts[0]; got.PartName != "فلتر زيت" {
		t.Errorf("unexpected manual product: %+v", got)
	}
	if got := payload.ManualProducts[0]; got.CostPrice != "20" || got.ShelfNumber != "A1" {
		t.Errorf("manual item should retain cost price and shelf number, got %+v", got)
	}
}

// TestBuildPurchaseBillPayload_MixedSelectedAndManual verifies that the
// unified rows separate a selected inventory item from an unselected item.
func TestBuildPurchaseBillPayload_MixedSelectedAndManual(t *testing.T) {
	form := url.Values{
		"store_id":              {"1"},
		"supplier_id":           {"2"},
		"products_product_id":   {"100", "0"},
		"products_track_stock":  {"true", "false"},
		"products_price":        {"50", "30"},
		"products_quantity":     {"2", "1"},
		"products_part_name":    {"فلتر مخزون", "فلتر يدوي"},
		"products_cost_price":   {"40", "20"},
		"products_shelf_number": {"A1", "B2"},
		"discount":              {"0"},
		"total_amount":          {"130"},
	}

	req, _ := http.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildPurchaseBillPayload(req)

	jsonBytes, _ := json.MarshalIndent(payload, "", "  ")
	t.Logf("Payload JSON:\n%s", string(jsonBytes))

	if len(payload.Products) != 1 {
		t.Errorf("expected exactly 1 stock product, got %d", len(payload.Products))
	}

	// Exactly 1 manual product
	if len(payload.ManualProducts) != 1 {
		t.Errorf("expected exactly 1 manual product, got %d", len(payload.ManualProducts))
	}

	if len(payload.Products) > 0 && !payload.Products[0].TrackStock {
		t.Error("store product must be marked for stock tracking")
	}
	if len(payload.ManualProducts) > 0 {
		mp := payload.ManualProducts[0]
		if mp.PartName != "فلتر يدوي" {
			t.Errorf("expected manual part name 'فلتر يدوي', got '%s'", mp.PartName)
		}
		if mp.CostPrice != "20" || mp.ShelfNumber != "B2" {
			t.Errorf("manual item should preserve cost and shelf data, got %+v", mp)
		}
	}
}

// TestBuildPurchaseBillPayload_MixedProducts keeps the legacy manual fields
// compatible with the unified rows while older clients are still in use.
func TestBuildPurchaseBillPayload_MixedProducts(t *testing.T) {
	form := url.Values{
		"store_id":             {"1"},
		"supplier_id":          {"2"},
		"products_product_id":  {"100"},
		"products_track_stock": {"true"},
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
		if mp.PartName != "فلتر يدوي" {
			t.Errorf("expected manual part name 'فلتر يدوي', got '%s'", mp.PartName)
		}
		if mp.Price != "30" {
			t.Errorf("expected manual price '30', got '%s'", mp.Price)
		}
		if mp.Quantity != "1" {
			t.Errorf("expected manual quantity '1', got '%s'", mp.Quantity)
		}
	}
}
