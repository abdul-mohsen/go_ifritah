package helpers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestBuildPurchaseBillPayload_SellingPriceOverride verifies that a
// non-blank products_selling_price row is carried through to the tracked
// product as an optional override, and that a blank value round-trips as
// "no override" (nil) rather than "set the price to 0" — the backend only
// honors this for admin/manager submissions, but the frontend's job is just
// to forward whatever was actually typed.
func TestBuildPurchaseBillPayload_SellingPriceOverride(t *testing.T) {
	form := url.Values{
		"store_id":               {"1"},
		"supplier_id":            {"2"},
		"products_product_id":    {"100", "200"},
		"products_track_stock":   {"true", "true"},
		"products_price":         {"50", "30"},
		"products_quantity":      {"2", "1"},
		"products_part_name":     {"فلتر مخزون", "فلتر آخر"},
		"products_cost_price":    {"40", "20"},
		"products_shelf_number":  {"A1", "B2"},
		"products_selling_price": {"75", ""},
		"discount":               {"0"},
		"total_amount":           {"130"},
	}

	req, _ := http.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildPurchaseBillPayload(req)

	if len(payload.Products) != 2 {
		t.Fatalf("expected 2 tracked products, got %d", len(payload.Products))
	}
	if payload.Products[0].SellingPrice == nil || *payload.Products[0].SellingPrice != "75" {
		t.Errorf("row 0: SellingPrice = %v, want \"75\"", payload.Products[0].SellingPrice)
	}
	if payload.Products[1].SellingPrice != nil {
		t.Errorf("row 1: SellingPrice = %v, want nil (blank input must not force a price of 0)", *payload.Products[1].SellingPrice)
	}
}

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

// TestBuildPurchaseBillPayload_TypedItemIsInventoryCandidate verifies that a
// typed item is sent through the backend product resolver even without an ID.
func TestBuildPurchaseBillPayload_TypedItemIsInventoryCandidate(t *testing.T) {
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

	if len(payload.Products) != 1 {
		t.Fatalf("expected one inventory candidate, got %d", len(payload.Products))
	}
	if len(payload.ManualProducts) != 0 {
		t.Fatalf("expected no manual products, got %d", len(payload.ManualProducts))
	}
	if got := payload.Products[0]; got.PartName != "فلتر زيت" {
		t.Errorf("unexpected inventory candidate: %+v", got)
	}
	if got := payload.Products[0]; got.CostPrice != "20" || got.ShelfNumber != "A1" {
		t.Errorf("inventory candidate should retain cost price and shelf number, got %+v", got)
	}
	if !payload.Products[0].TrackStock {
		t.Error("typed item must be marked for stock tracking")
	}
}

// TestBuildPurchaseBillPayload_MixedRowsAreInventoryCandidates verifies that
// selected and typed rows share the same backend product-sync path.
func TestBuildPurchaseBillPayload_MixedRowsAreInventoryCandidates(t *testing.T) {
	form := url.Values{
		"store_id":              {"1"},
		"supplier_id":           {"2"},
		"products_product_id":   {"100", "0"},
		"products_track_stock":  {"true", "true"},
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

	if len(payload.Products) != 2 {
		t.Errorf("expected exactly 2 inventory candidates, got %d", len(payload.Products))
	}

	if len(payload.ManualProducts) != 0 {
		t.Errorf("expected no manual products, got %d", len(payload.ManualProducts))
	}

	for i, product := range payload.Products {
		if !product.TrackStock {
			t.Errorf("products[%d] must be marked for stock tracking", i)
		}
	}
	if got := payload.Products[1]; got.PartName != "فلتر يدوي" {
		t.Errorf("expected typed part name in second inventory candidate, got '%s'", got.PartName)
	}
	if got := payload.Products[1]; got.CostPrice != "20" || got.ShelfNumber != "B2" {
		t.Errorf("typed item should preserve cost and shelf data, got %+v", got)
	}
}

func TestBuildPurchaseBillPayload_ExplicitLegacyManualRowStaysManual(t *testing.T) {
	form := url.Values{
		"store_id":              {"1"},
		"supplier_id":           {"2"},
		"products_product_id":   {"0"},
		"products_track_stock":  {"false"},
		"products_part_name":    {"Service charge"},
		"products_price":        {"30"},
		"products_quantity":     {"1"},
		"products_cost_price":   {"30"},
		"products_shelf_number": {"A1"},
	}

	req, _ := http.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildPurchaseBillPayload(req)
	if len(payload.Products) != 0 || len(payload.ManualProducts) != 1 {
		t.Fatalf("explicit legacy manual row was not preserved: products=%d manual=%d",
			len(payload.Products), len(payload.ManualProducts))
	}
	if payload.ManualProducts[0].PartName != "Service charge" {
		t.Errorf("manual row name = %q, want %q", payload.ManualProducts[0].PartName, "Service charge")
	}
}

func TestBuildPurchaseBillPayloadFallsBackToCostPricePerRow(t *testing.T) {
	form := url.Values{
		"store_id":             {"1"},
		"supplier_id":          {"2"},
		"products_product_id":  {"100", "0"},
		"products_track_stock": {"true", "true"},
		"products_price":       {"50", ""},
		"products_quantity":    {"2", "1"},
		"products_part_name":   {"Existing item", "Typed item"},
		"products_cost_price":  {"40", "20"},
		"total_amount":         {"120"},
	}

	req, _ := http.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := BuildPurchaseBillPayload(req)
	if len(payload.Products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(payload.Products))
	}
	if payload.Products[1].Price != "20" {
		t.Errorf("legacy row without products_price should use cost price, got %q", payload.Products[1].Price)
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
