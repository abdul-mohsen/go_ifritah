package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleBranchesSearchArabicNormalization verifies the new Arabic-aware
// matcher handles harakat, alef-folding, ta-marbuta, and Indic digits when
// users type queries against the branches list page.
func TestHandleBranchesSearchArabicNormalization(t *testing.T) {
	cases := []struct {
		name         string
		query        string
		mustContain  []string
		mustNotShow  []string
	}{
		{
			name:        "harakat-stripped",
			query:       "جَدّة",
			mustContain: []string{"فرع جدة"},
			mustNotShow: []string{"فرع الدمام", "فرع مكة"},
		},
		{
			name:        "ta-marbuta-folds-to-ha",
			query:       "مكه",
			mustContain: []string{"فرع مكة"},
			mustNotShow: []string{"فرع جدة", "فرع الدمام"},
		},
		{
			name:        "indic-digits-on-phone",
			query:       "٠١٢٢", // matches 0122 in Jeddah branch phone
			mustContain: []string{"فرع جدة"},
			mustNotShow: []string{"فرع الدمام"},
		},
		{
			name:        "manager-name-search",
			query:       "الدوسري",
			mustContain: []string{"الفرع الرئيسي"},
			mustNotShow: []string{"فرع جدة"},
		},
		{
			name:        "two-tokens-AND",
			query:       "فرع جدة",
			mustContain: []string{"فرع جدة"},
			mustNotShow: []string{"فرع الدمام", "فرع مكة"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seedTestSession()
			setupBranchBackend(t)

			req, _ := http.NewRequest("GET", "/dashboard/branches?q="+c.query, nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
			req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

			rr := httptest.NewRecorder()
			http.HandlerFunc(HandleBranches).ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status %d", rr.Code)
			}
			body := rr.Body.String()
			for _, want := range c.mustContain {
				if !strings.Contains(body, want) {
					t.Errorf("query %q expected to find %q in rendered body", c.query, want)
				}
			}
			for _, banned := range c.mustNotShow {
				if strings.Contains(body, banned) {
					t.Errorf("query %q should NOT have shown %q", c.query, banned)
				}
			}
		})
	}
}
