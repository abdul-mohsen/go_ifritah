package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"afrita/config"
	"afrita/helpers"
)

// productAutocompleteBackend returns a httptest server that answers
// /api/v2/product/all with a fixed catalogue covering the Arabic edge cases
// the BE matcher must handle.
//
// On feat/search-and-filters, search is 100% backend-driven, so the mock
// BE applies the Arabic-aware match (helpers.MatchSearchQuery) instead of
// the FE.
func productAutocompleteBackend(t *testing.T) {
	t.Helper()
	type catalogItem struct {
		ID          int    `json:"id"`
		PartName    string `json:"part_name"`
		Name        string `json:"name"`
		Price       string `json:"price"`
		Quantity    string `json:"quantity"`
		ArticleID   int    `json:"article_id"`
		ShelfNumber string `json:"shelf_number"`
	}
	catalog := []catalogItem{
		{ID: 1, PartName: "فِلْتَر زيت", Name: "Oil Filter", Price: "50", Quantity: "10", ArticleID: 9001, ShelfNumber: "A1"},
		{ID: 2, PartName: "إطار خارجي", Name: "Tyre", Price: "300", Quantity: "5", ArticleID: 9002, ShelfNumber: "B2"},
		{ID: 3, PartName: "مصباح أمامي", Name: "Headlight", Price: "120", Quantity: "3", ArticleID: 9003, ShelfNumber: "C3"},
		{ID: 4, PartName: "بطارية", Name: "Battery", Price: "450", Quantity: "8", ArticleID: 9004, ShelfNumber: "D4"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v2/product/all" {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			q, _ := payload["query"].(string)
			filtered := catalog
			if q != "" {
				filtered = filtered[:0]
				for _, p := range catalog {
					idStr := strconv.Itoa(p.ID)
					articleStr := strconv.Itoa(p.ArticleID)
					if helpers.MatchSearchQuery(q, p.PartName, p.Name, idStr, articleStr, p.ShelfNumber) {
						filtered = append(filtered, p)
					}
				}
			}
			resp := map[string]interface{}{"data": filtered}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)
	prev := config.BackendDomain
	config.BackendDomain = srv.URL
	t.Cleanup(func() { config.BackendDomain = prev })
}

// TestHandleProductsSearchJSON_ArabicAware proves the invoice / purchase-bill
// autocomplete honours Arabic folding, harakat stripping, Indic digits, and
// article_id lookups now that it goes through helpers.MatchSearchQuery.
func TestHandleProductsSearchJSON_ArabicAware(t *testing.T) {
	cases := []struct {
		name        string
		query       string
		mustHaveIDs []int
		mustNotHave []int
	}{
		{
			name:        "harakat-stripped-from-query",
			query:       "فلتر",
			mustHaveIDs: []int{1},
			mustNotHave: []int{2, 3, 4},
		},
		{
			name:        "alef-folded",
			query:       "اطار",
			mustHaveIDs: []int{2},
			mustNotHave: []int{1, 3, 4},
		},
		{
			name:        "english-name-still-matches",
			query:       "battery",
			mustHaveIDs: []int{4},
			mustNotHave: []int{1, 2, 3},
		},
		{
			name:        "article-id-exact",
			query:       "9003",
			mustHaveIDs: []int{3},
			mustNotHave: []int{1, 2, 4},
		},
		{
			name:        "indic-digits-on-article-id",
			query:       "٩٠٠٤",
			mustHaveIDs: []int{4},
			mustNotHave: []int{1, 2, 3},
		},
		{
			name:        "shelf-number",
			query:       "B2",
			mustHaveIDs: []int{2},
			mustNotHave: []int{1, 3, 4},
		},
		{
			name:        "empty-query-returns-all",
			query:       "",
			mustHaveIDs: []int{1, 2, 3, 4},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seedTestSession()
			productAutocompleteBackend(t)

			form := url.Values{}
			form.Set("query", c.query)
			req, _ := http.NewRequest("POST", "/api/products/search-json", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
			req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

			rr := httptest.NewRecorder()
			http.HandlerFunc(HandleProductsSearchJSON).ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
			}
			var got []map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
			}
			gotIDs := map[int]bool{}
			for _, item := range got {
				if id, ok := item["id"].(float64); ok {
					gotIDs[int(id)] = true
				}
			}
			for _, want := range c.mustHaveIDs {
				if !gotIDs[want] {
					t.Errorf("query %q expected id=%d in response, got %v", c.query, want, gotIDs)
				}
			}
			for _, banned := range c.mustNotHave {
				if gotIDs[banned] {
					t.Errorf("query %q should NOT have returned id=%d, got %v", c.query, banned, gotIDs)
				}
			}
		})
	}
}
