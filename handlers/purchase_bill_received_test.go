package handlers

import (
	"afrita/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestPurchaseBillReceiptProxyMarksAndUnmarksBill(t *testing.T) {
	const billID = "77"
	received := false
	var mutationMethods []string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/purchase_bill/" + billID + "/received":
			mutationMethods = append(mutationMethods, r.Method)
			if r.Method == http.MethodPut {
				received = true
			} else if r.Method == http.MethodDelete {
				received = false
			}
			_, _ = w.Write([]byte(`{"detail":"success"}`))
		case "/api/v2/purchase_bill/" + billID:
			detail := `{"id":77,"sequence_number":12,"supplier_sequence_number":34,"bill_type":5,"state":0,"effective_date":"2026-04-09T22:30:00Z","products":[]}`
			if received {
				detail = `{"id":77,"sequence_number":12,"supplier_sequence_number":34,"bill_type":5,"state":0,"effective_date":"2026-04-09T22:30:00Z","received_at":{"Time":"2026-04-09T22:30:00Z","Valid":true},"products":[]}`
			}
			_, _ = w.Write([]byte(detail))
		default:
			t.Fatalf("unexpected backend request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer backend.Close()

	originalDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = originalDomain }()

	cleanup := setupPBTestSession("pb-received-proxy", "pb-received-token")
	defer cleanup()

	call := func(method string) string {
		req := httptest.NewRequest(method, "/api/purchase-bills/"+billID+"/received", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-received-proxy"})
		req = mux.SetURLVars(req, map[string]string{"id": billID})
		w := httptest.NewRecorder()

		if method == http.MethodPut {
			HandleMarkPurchaseBillReceived(w, req)
		} else {
			HandleUnmarkPurchaseBillReceived(w, req)
		}
		if w.Code != http.StatusOK {
			t.Fatalf("%s proxy returned %d: %s", method, w.Code, w.Body.String())
		}
		return w.Body.String()
	}

	markedBody := call(http.MethodPut)
	if !strings.Contains(markedBody, `id="bill-header"`) {
		t.Fatal("mark response did not render the bill header fragment")
	}
	if !strings.Contains(markedBody, "Received on") && !strings.Contains(markedBody, "تم الاستلام في") {
		t.Fatal("mark response did not show the receipt confirmation")
	}
	if !strings.Contains(markedBody, "2026-04-10") {
		t.Fatalf("mark response did not localize the receipt date: %s", markedBody)
	}

	unmarkedBody := call(http.MethodDelete)
	if strings.Contains(unmarkedBody, "Received on") || strings.Contains(unmarkedBody, "تم الاستلام في") {
		t.Fatal("unmark response still showed the receipt confirmation")
	}

	if len(mutationMethods) != 2 || mutationMethods[0] != http.MethodPut || mutationMethods[1] != http.MethodDelete {
		t.Fatalf("backend mutation methods = %v, want PUT then DELETE", mutationMethods)
	}
}
