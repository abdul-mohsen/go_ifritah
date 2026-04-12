package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"afrita/config"
	"afrita/models"
)

func TestHandleInvoicesPaginationLinks(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root = filepath.Dir(root)
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(filepath.Dir(root))
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/bill/all" {
			http.NotFound(w, r)
			return
		}

		items := make([]models.Invoice, 0, 30)
		for i := 0; i < 30; i++ {
			items = append(items, models.Invoice{
				ID:             i + 1,
				SequenceNumber: 1000 + i,
				Total:          100.0,
				TotalBeforeVAT: 100.0,
				TotalVAT:       0.0,
				Discount:       0.0,
				State:          3,
				EffectiveDate: struct {
					Time  string `json:"Time"`
					Valid bool   `json:"Valid"`
				}{Time: time.Now().Format(time.RFC3339), Valid: true},
			})
		}

		payload, _ := json.Marshal(items)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	prevBackend := config.BackendDomain
	config.BackendDomain = server.URL
	t.Cleanup(func() { config.BackendDomain = prevBackend })

	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "test-session")
		config.SessionTokensMutex.Unlock()
	})

	req := httptest.NewRequest("GET", "/dashboard/invoices?page=1&per=10", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleInvoices(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "page=0") {
		t.Fatalf("expected previous page link to include page=0")
	}
	if !strings.Contains(body, "page=2") {
		t.Fatalf("expected next page link to include page=2")
	}
	if !strings.Contains(body, "الصفحة 2") {
		t.Fatalf("expected current page label to be 2")
	}
}
