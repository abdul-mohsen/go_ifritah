package handlers

import (
	"afrita/config"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandleCreateProductSendsFreeTextNameWithoutFabricatedID(t *testing.T) {
	var received struct {
		StoreID  int `json:"store_id"`
		Products []struct {
			ProductID *int   `json:"product_id"`
			Name      string `json:"name"`
		} `json:"products"`
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/product" {
			t.Fatalf("unexpected backend path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode product payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	originalDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = originalDomain }()

	cleanup := setupPBTestSession("product-free-text", "product-free-text-token")
	defer cleanup()

	form := url.Values{
		"store_id":       {"1"},
		"part_name[]":    {"Generic brake pad"},
		"quantity[]":     {"2"},
		"price[]":        {"15"},
		"cost_price[]":   {"10"},
		"shelf_number[]": {"A1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/products/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "product-free-text"})
	w := httptest.NewRecorder()

	HandleCreateProduct(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if received.StoreID != 1 || len(received.Products) != 1 {
		t.Fatalf("unexpected payload: %+v", received)
	}
	if received.Products[0].ProductID != nil {
		t.Fatalf("expected free-text product to omit product_id, got %v", *received.Products[0].ProductID)
	}
	if received.Products[0].Name != "Generic brake pad" {
		t.Fatalf("name = %q, want %q", received.Products[0].Name, "Generic brake pad")
	}
}
