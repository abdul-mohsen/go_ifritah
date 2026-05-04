package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"afrita/config"
)

// productAutocompleteBackend returns a httptest server that answers
// /api/v2/product/all with a fixed catalogue covering the Arabic edge cases
// the FE matcher must handle.
func productAutocompleteBackend(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v2/product/all" {
			_, _ = w.Write([]byte(`{"data":[
				{"id":1,"part_name":"فِلْتَر زيت","name":"Oil Filter","price":"50","quantity":"10","article_id":9001,"shelf_number":"A1"},
				{"id":2,"part_name":"إطار خارجي","name":"Tyre","price":"300","quantity":"5","article_id":9002,"shelf_number":"B2"},
				{"id":3,"part_name":"مصباح أمامي","name":"Headlight","price":"120","quantity":"3","article_id":9003,"shelf_number":"C3"},
				{"id":4,"part_name":"بطارية","name":"Battery","price":"450","quantity":"8","article_id":9004,"shelf_number":"D4"}
			]}`))
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
