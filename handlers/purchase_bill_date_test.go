package handlers

import (
	"afrita/config"
	"afrita/helpers"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// Regression coverage for the "purchase bill date shifts back a day" bug.
//
// The backend may return effective_date as a UTC-offset timestamp. A wall
// clock time of 2026-04-10 01:30 in Riyadh (+03:00) is 2026-04-09 22:30 UTC.
// The buggy code used to slice the first 10 characters of the raw string
// (i.e. of the UTC representation), which yields "2026-04-09" - one
// calendar day earlier than what the user actually set. The fix
// re-localizes to Asia/Riyadh via helpers.ToDisplayDate before taking the
// calendar date.
const (
	pbDateUTCInstant    = "2026-04-09T22:30:00Z" // same instant as below, in UTC
	pbDateRiyadhCal     = "2026-04-10"           // correct Riyadh calendar date
	pbDateWrongIfSliced = "2026-04-09"           // what naive [:10] slicing used to produce
)

func TestExtractDateFieldReLocalizesUTCToRiyadh(t *testing.T) {
	// Plain string shape.
	if got := extractDateField(pbDateUTCInstant); got != pbDateRiyadhCal {
		t.Fatalf("extractDateField(%q) = %q, want %q (naive slicing would give %q)",
			pbDateUTCInstant, got, pbDateRiyadhCal, pbDateWrongIfSliced)
	}

	// {Time, Valid} wrapper shape (what the backend returns for sql.NullTime-like fields).
	wrapped := map[string]interface{}{"Time": pbDateUTCInstant, "Valid": true}
	if got := extractDateField(wrapped); got != pbDateRiyadhCal {
		t.Fatalf("extractDateField(wrapped %q) = %q, want %q (naive slicing would give %q)",
			pbDateUTCInstant, got, pbDateRiyadhCal, pbDateWrongIfSliced)
	}
}

// TestEditPurchaseBillDatePreservesCalendarDay verifies the edit page's date
// input is pre-filled with the Riyadh calendar date, not the UTC one.
func TestEditPurchaseBillDatePreservesCalendarDay(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		if strings.Contains(path, "/api/v2/store/all") ||
			strings.Contains(path, "/api/v2/supplier/all") ||
			strings.Contains(path, "/api/v2/product/all") ||
			strings.Contains(path, "/api/v2/purchase_bill/all") {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}

		if strings.Contains(path, "/api/v2/purchase_bill/") {
			detail := map[string]interface{}{
				"id":              "654",
				"bill_type":       5,
				"state":           0,
				"sub_total":       50.0,
				"discount":        0,
				"products":        []interface{}{},
				"manual_products": []interface{}{},
				"store_id":        1,
				"effective_date": map[string]interface{}{
					"Time":  pbDateUTCInstant,
					"Valid": true,
				},
			}
			_ = json.NewEncoder(w).Encode(detail)
			return
		}

		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-date-edit-test", "pb-date-edit-token")
	defer cleanup()

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills/edit/654", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-date-edit-test"})
	req = mux.SetURLVars(req, map[string]string{"id": "654"})
	w := httptest.NewRecorder()

	HandleEditPurchaseBill(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", w.Code, body[:min(500, len(body))])
	}

	if strings.Contains(body, `value="`+pbDateWrongIfSliced+`"`) {
		t.Fatalf("edit purchase bill page shows the wrong (UTC-shifted) date %q - the date shifted back a day", pbDateWrongIfSliced)
	}
	if !strings.Contains(body, `value="`+pbDateRiyadhCal+`"`) {
		t.Fatalf("expected edit purchase bill page date input to show %q (Riyadh calendar date), got body containing:\n%.800s", pbDateRiyadhCal, body)
	}
}

// TestPurchaseBillDetailDatePreservesCalendarDay covers the same bug on the
// read-only detail page.
func TestPurchaseBillDetailDatePreservesCalendarDay(t *testing.T) {
	helpers.APICache.Delete("suppliers")
	helpers.APICache.Delete("stores")

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		if strings.Contains(path, "/api/v2/store/all") || strings.Contains(path, "/api/v2/supplier/all") {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}

		if strings.Contains(path, "/api/v2/purchase_bill/") {
			detail := map[string]interface{}{
				"id":        "655",
				"bill_type": 5,
				"state":     0,
				"sub_total": 100.0,
				"discount":  0,
				"total_vat": 15.0,
				"total":     115.0,
				"products":  []interface{}{},
				"store_id":  1,
				"effective_date": map[string]interface{}{
					"Time":  pbDateUTCInstant,
					"Valid": true,
				},
			}
			_ = json.NewEncoder(w).Encode(detail)
			return
		}

		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-date-detail-test", "pb-date-detail-token")
	defer cleanup()

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills/655", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-date-detail-test"})
	req = mux.SetURLVars(req, map[string]string{"id": "655"})
	w := httptest.NewRecorder()

	HandleGetPurchaseBill(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", w.Code, body[:min(500, len(body))])
	}

	if strings.Contains(body, pbDateWrongIfSliced) {
		t.Fatalf("purchase bill detail page shows the wrong (UTC-shifted) date %q", pbDateWrongIfSliced)
	}
	if !strings.Contains(body, pbDateRiyadhCal) {
		t.Fatalf("expected purchase bill detail page to show %q (Riyadh calendar date), got body containing:\n%.800s", pbDateRiyadhCal, body)
	}
}
