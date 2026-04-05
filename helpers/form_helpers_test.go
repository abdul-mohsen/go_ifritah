package helpers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestBuildPurchaseBillPayload_ManualProducts verifies that BuildPurchaseBillPayload
// converts manual-only products into products[] with random IDs (backend requires non-empty products[]).
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

	// Manual items should still be in ManualProducts
	if len(payload.ManualProducts) < 2 {
		t.Errorf("expected at least 2 manual_products, got %d", len(payload.ManualProducts))
	}
	for i, p := range payload.ManualProducts {
		if p.PartName == "" {
			t.Errorf("manual_product[%d] should have a part_name set", i)
		}
	}
	// Products should also be populated (converted from manual) since backend requires non-empty products[]
	if len(payload.Products) < 2 {
		t.Errorf("products should contain converted manual items, got %d", len(payload.Products))
	}
	for i, p := range payload.Products {
		if p.ID == 0 {
			t.Errorf("products[%d] should have a non-zero random ID", i)
		}
		if p.PartName == "" {
			t.Errorf("products[%d] should have a part_name set", i)
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
