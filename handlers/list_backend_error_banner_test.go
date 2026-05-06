package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"
	"afrita/helpers"
)

// TestListPagesShowBackendErrorBanner verifies that when the upstream backend
// is returning 5xx for list endpoints, every list page renders a visible
// error banner (role="alert" with the localised Arabic message) instead of
// an empty-but-success page that gives the user no signal that something
// went wrong.
//
// This is the regression test for the bug observed against dev.ifritah.com
// where /api/v2/bill/all and /api/v2/product/all returned 500 but the FE
// rendered an empty-state page with no indication. It guards every list
// handler that talks to the backend list APIs.
func TestListPagesShowBackendErrorBanner(t *testing.T) {
	// Mock backend that returns 500 for every JSON list endpoint we care about.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"simulated upstream failure"}`))
	}))
	t.Cleanup(backend.Close)

	prevBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = prevBackend })

	// Fresh session; bypass token middleware.
	const sid = "list-banner-test"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sid] = "mock-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sid)
		config.SessionTokensMutex.Unlock()
	})

	// Drop any caches a previous test may have populated. Otherwise a cached
	// success would mask the upstream 500 and the banner would not render.
	for _, k := range []string{
		"invoices_all", "purchase_bills", "products", "clients",
		"suppliers", "stores", "orders",
	} {
		helpers.APICache.Delete(k)
	}

	cases := []struct {
		name    string
		path    string
		handler http.HandlerFunc
		// One of these substrings must appear in the rendered HTML.
		bannerText string
	}{
		{"invoices", "/dashboard/invoices", HandleInvoices, "تعذر تحميل الفواتير"},
		{"purchase-bills", "/dashboard/purchase-bills", HandlePurchaseBills, "تعذر تحميل فواتير المشتريات"},
		{"products", "/dashboard/products", HandleProducts, "تعذر تحميل المنتجات"},
		{"clients", "/dashboard/clients", HandleClients, "تعذر تحميل العملاء"},
		{"suppliers", "/dashboard/suppliers", HandleSuppliers, "تعذر تحميل الموردين"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Drop the per-page cache again before each subtest so a sibling
			// subtest's success cannot mask this one's expected failure.
			for _, k := range []string{
				"invoices_all", "purchase_bills", "products", "clients",
				"suppliers", "stores", "orders",
			} {
				helpers.APICache.Delete(k)
			}

			req := httptest.NewRequest("GET", tc.path, nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: sid})
			w := httptest.NewRecorder()
			tc.handler(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status %d (want 200 soft-fail)\nBody: %.300s", w.Code, w.Body.String())
			}
			body := w.Body.String()
			if !strings.Contains(body, `role="alert"`) {
				t.Errorf("no role=\"alert\" element in rendered HTML — banner missing\nBody snippet:\n%.500s", body)
			}
			if !strings.Contains(body, tc.bannerText) {
				t.Errorf("expected banner text %q not found in rendered HTML\nBody snippet:\n%.800s", tc.bannerText, body)
			}
		})
	}
}
