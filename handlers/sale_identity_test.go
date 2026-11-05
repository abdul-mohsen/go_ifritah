package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"afrita/config"
	"afrita/helpers"

	"github.com/gorilla/mux"
)

const (
	saleIdentityPosID   = "3f0c6a1e-8d2b-4c4e-9a7f-1b2c3d4e5f60"
	saleIdentityAttempt = "9b1d2c3e-4f5a-4b6c-8d7e-0f1a2b3c4d5e"
	saleIdentityNext    = "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
)

// saleDraftBill is draft 321, which the submit and edit handlers read before they write it back.
var saleDraftBill = map[string]interface{}{
	"id":                321,
	"sequence_number":   9001,
	"state":             0,
	"total":             115.0,
	"discount":          5.0,
	"store_id":          12,
	"branch_id":         7,
	"client_id":         44,
	"payment_method":    2,
	"effective_date":    "2025-02-03T00:00:00+03:00",
	"payment_due_date":  "2025-02-10T00:00:00+03:00",
	"deliver_date":      "2025-02-04T00:00:00+03:00",
	"maintenance_cost":  3.5,
	"vin":               "VIN-SALE-321",
	"user_name":         "Sale Customer",
	"user_phone_number": "0500000000",
	"note":              "draft for the sale identity",
	"products":          []interface{}{},
	"manual_products":   []map[string]interface{}{{"name": "Manual service", "part_name": "SRV-1", "price": 100, "quantity": 1}},
}

// billWrites records the bill bodies that a handler sends to the backend.
type billWrites struct {
	mu     sync.Mutex
	bodies []map[string]interface{}
}

func (w *billWrites) add(body map[string]interface{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bodies = append(w.bodies, body)
}

func (w *billWrites) all() []map[string]interface{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]map[string]interface{}(nil), w.bodies...)
}

// startSaleBackend fakes the bill routes that the sale handlers call. Bill 321 is the draft above. Every bill write is
// recorded, and it is answered with bill id 321.
func startSaleBackend(t *testing.T) *billWrites {
	t.Helper()
	return startSaleBackendWithBill(t, saleDraftBill)
}

// startSaleBackendWithBill is startSaleBackend with bill 321 served from bill instead of saleDraftBill.
func startSaleBackendWithBill(t *testing.T, bill map[string]interface{}) *billWrites {
	t.Helper()
	writes := &billWrites{}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/bill/321":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(bill)
		case (r.Method == http.MethodPost && r.URL.Path == "/api/v2/bill") ||
			(r.Method == http.MethodPut && r.URL.Path == "/api/v2/bill/321"):
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode bill write: %v", err)
			}
			writes.add(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("321"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(backend.Close)

	prevBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = prevBackend })
	return writes
}

// seedSaleSession registers a session for the sale handler tests and returns its cookie.
func seedSaleSession(t *testing.T) *http.Cookie {
	t.Helper()
	const sessionID = "sale-identity-test"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = "test-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	})
	return &http.Cookie{Name: "session_id", Value: sessionID}
}

// saleFormBody is the form of a final sale that the add and edit pages post.
func saleFormBody(attempt string) string {
	return url.Values{
		"store_id":            {"12"},
		"branch_id":           {"7"},
		"payment_method":      {"10"},
		"state":               {"1"},
		"discount":            {"0"},
		"maintenance_cost":    {"0"},
		"total_amount":        {"115"},
		"products_product_id": {"6"},
		"products_price":      {"100"},
		"products_quantity":   {"1"},
		"pos_id":              {saleIdentityPosID},
		"bill_request_id":     {attempt},
	}.Encode()
}

// assertSaleIdentity checks that a bill write carries the pos_id and bill_request_id that the form sent.
func assertSaleIdentity(t *testing.T, body map[string]interface{}, posID, attempt string) {
	t.Helper()
	if got := body["pos_id"]; got != posID {
		t.Errorf("pos_id = %v, want %q", got, posID)
	}
	if got := body["bill_request_id"]; got != attempt {
		t.Errorf("bill_request_id = %v, want %q", got, attempt)
	}
}

func postSaleForm(t *testing.T, handler http.HandlerFunc, method, path, body string, vars map[string]string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func TestCreateInvoiceSendsSaleIdentity(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)

	w := postSaleForm(t, HandleCreateInvoice, http.MethodPost, "/api/invoices", saleFormBody(saleIdentityAttempt), nil, cookie)

	if w.Code != http.StatusOK {
		t.Fatalf("create status = %d body=%q, want 200", w.Code, w.Body.String())
	}
	bodies := writes.all()
	if len(bodies) != 1 {
		t.Fatalf("backend got %d bill writes, want 1", len(bodies))
	}
	assertSaleIdentity(t, bodies[0], saleIdentityPosID, saleIdentityAttempt)
}

func TestCreateInvoiceRetrySendsTheSameBillRequestID(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)

	// A double click or a retry posts the same attempt again. The next sale has its own attempt.
	for _, attempt := range []string{saleIdentityAttempt, saleIdentityAttempt, saleIdentityNext} {
		w := postSaleForm(t, HandleCreateInvoice, http.MethodPost, "/api/invoices", saleFormBody(attempt), nil, cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("create with attempt %s status = %d body=%q, want 200", attempt, w.Code, w.Body.String())
		}
	}

	bodies := writes.all()
	if len(bodies) != 3 {
		t.Fatalf("backend got %d bill writes, want 3", len(bodies))
	}
	assertSaleIdentity(t, bodies[0], saleIdentityPosID, saleIdentityAttempt)
	assertSaleIdentity(t, bodies[1], saleIdentityPosID, saleIdentityAttempt)
	assertSaleIdentity(t, bodies[2], saleIdentityPosID, saleIdentityNext)
}

func TestSubmitDraftInvoiceSendsSaleIdentity(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)

	form := url.Values{"pos_id": {saleIdentityPosID}, "bill_request_id": {saleIdentityAttempt}}.Encode()
	w := postSaleForm(t, HandleSubmitDraftInvoice, http.MethodPost, "/api/invoices/321/submit", form, map[string]string{"id": "321"}, cookie)

	if w.Code != http.StatusOK {
		t.Fatalf("submit status = %d body=%q, want 200", w.Code, w.Body.String())
	}
	bodies := writes.all()
	if len(bodies) != 1 {
		t.Fatalf("backend got %d bill writes, want 1", len(bodies))
	}
	assertSaleIdentity(t, bodies[0], saleIdentityPosID, saleIdentityAttempt)
	if got := bodies[0]["state"]; got != float64(1) {
		t.Errorf("submitted state = %v, want 1", got)
	}
}

func TestSubmitDraftInvoiceForwardsLineDiscounts(t *testing.T) {
	bill := make(map[string]interface{}, len(saleDraftBill))
	for key, value := range saleDraftBill {
		bill[key] = value
	}
	bill["products"] = []interface{}{map[string]interface{}{"product_id": 6, "name": "Product six", "part_name": "PART-6", "price": 100, "quantity": 1, "discount": 5}}
	bill["manual_products"] = []interface{}{map[string]interface{}{"name": "Manual service", "part_name": "SRV-1", "price": 100, "quantity": 1, "discount": 1.5}}
	writes := startSaleBackendWithBill(t, bill)
	cookie := seedSaleSession(t)

	form := url.Values{"pos_id": {saleIdentityPosID}, "bill_request_id": {saleIdentityAttempt}}.Encode()
	w := postSaleForm(t, HandleSubmitDraftInvoice, http.MethodPost, "/api/invoices/321/submit", form, map[string]string{"id": "321"}, cookie)

	if w.Code != http.StatusOK {
		t.Fatalf("submit status = %d body=%q, want 200", w.Code, w.Body.String())
	}
	bodies := writes.all()
	if len(bodies) != 1 {
		t.Fatalf("backend got %d bill writes, want 1", len(bodies))
	}
	assertLineDiscount(t, bodies[0], "products", 0, "5")
	assertLineDiscount(t, bodies[0], "manual_products", 0, "1.5")
}

func TestUpdateInvoiceSendsSaleIdentity(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)

	w := postSaleForm(t, HandleUpdateInvoice, http.MethodPut, "/api/invoices/321", saleFormBody(saleIdentityAttempt), map[string]string{"id": "321"}, cookie)

	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%q, want 200", w.Code, w.Body.String())
	}
	bodies := writes.all()
	if len(bodies) != 1 {
		t.Fatalf("backend got %d bill writes, want 1", len(bodies))
	}
	assertSaleIdentity(t, bodies[0], saleIdentityPosID, saleIdentityAttempt)
}

// TestSaleFormsCarrySaleIdentityScope checks that the pages that post a final sale mark their submits with a scope.
// static/js/sale-identity.js adds pos_id and bill_request_id to every submit that has a scope.
// Pages with line discounts also mark their form with data-sale-discounts, placed right after the scope.
func TestSaleFormsCarrySaleIdentityScope(t *testing.T) {
	startSaleBackend(t)
	cookie := seedSaleSession(t)

	pages := []struct {
		name          string
		handler       http.HandlerFunc
		path          string
		vars          map[string]string
		want          string
		wantDiscounts bool
	}{
		{"add-invoice", HandleAddInvoice, "/dashboard/invoices/add-invoice", nil, `data-sale-scope="new-sale"`, true},
		{"edit-invoice", HandleEditInvoice, "/dashboard/invoices/edit/321", map[string]string{"id": "321"}, `data-sale-scope="draft-321"`, true},
		{"invoice-detail", HandleGetInvoice, "/bill/321", map[string]string{"id": "321"}, `data-sale-scope="draft-321"`, false},
	}
	for _, page := range pages {
		t.Run(page.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, page.path, nil)
			req.AddCookie(cookie)
			if page.vars != nil {
				req = mux.SetURLVars(req, page.vars)
			}
			w := httptest.NewRecorder()
			page.handler(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("page status = %d, want 200", w.Code)
			}
			body := w.Body.String()
			if !strings.Contains(body, page.want) {
				t.Errorf("page does not contain %s", page.want)
			}
			if !strings.Contains(body, `/static/js/sale-identity.js`) {
				t.Errorf("page does not load /static/js/sale-identity.js")
			}
			if page.wantDiscounts && !strings.Contains(body, page.want+" data-sale-discounts") {
				t.Errorf("sale form does not carry data-sale-discounts after its scope")
			}
		})
	}
}

// saleFormWithManualLine is saleFormBody plus a manual service line without discount fields.
func saleFormWithManualLine(t *testing.T, attempt string) url.Values {
	t.Helper()
	values, err := url.ParseQuery(saleFormBody(attempt))
	if err != nil {
		t.Fatalf("parse sale form: %v", err)
	}
	values.Set("manual_part_name", "Manual service")
	values.Set("manual_price", "100")
	values.Set("manual_quantity", "1")
	return values
}

// saleFormWithLineDiscounts adds a discount to the product line and to the manual line of a final sale.
func saleFormWithLineDiscounts(t *testing.T, attempt string) string {
	t.Helper()
	values := saleFormWithManualLine(t, attempt)
	values.Set("products_discount", "5")
	values.Set("manual_discount", "1.5")
	return values.Encode()
}

// assertLineDiscount checks the discount sent for one line of list, which is "products" or "manual_products".
func assertLineDiscount(t *testing.T, body map[string]interface{}, list string, index int, want string) {
	t.Helper()
	lines, _ := body[list].([]interface{})
	if index >= len(lines) {
		t.Fatalf("%s has %d lines, want line %d", list, len(lines), index)
	}
	line, _ := lines[index].(map[string]interface{})
	if got := line["discount"]; got != want {
		t.Errorf("%s[%d] discount = %v, want %q", list, index, got, want)
	}
}

func TestSaleCreateForwardsLineDiscounts(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)

	w := postSaleForm(t, HandleCreateInvoice, http.MethodPost, "/api/invoices", saleFormWithLineDiscounts(t, saleIdentityAttempt), nil, cookie)

	if w.Code != http.StatusOK {
		t.Fatalf("create status = %d body=%q, want 200", w.Code, w.Body.String())
	}
	bodies := writes.all()
	if len(bodies) != 1 {
		t.Fatalf("backend got %d bill writes, want 1", len(bodies))
	}
	assertLineDiscount(t, bodies[0], "products", 0, "5")
	assertLineDiscount(t, bodies[0], "manual_products", 0, "1.5")
}

func TestSaleCreateSendsZeroDiscountWhenAbsent(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)

	w := postSaleForm(t, HandleCreateInvoice, http.MethodPost, "/api/invoices", saleFormWithManualLine(t, saleIdentityAttempt).Encode(), nil, cookie)

	if w.Code != http.StatusOK {
		t.Fatalf("create status = %d body=%q, want 200", w.Code, w.Body.String())
	}
	bodies := writes.all()
	if len(bodies) != 1 {
		t.Fatalf("backend got %d bill writes, want 1", len(bodies))
	}
	assertLineDiscount(t, bodies[0], "products", 0, "0")
	assertLineDiscount(t, bodies[0], "manual_products", 0, "0")
}

func TestUpdateInvoiceForwardsLineDiscounts(t *testing.T) {
	writes := startSaleBackend(t)
	cookie := seedSaleSession(t)

	w := postSaleForm(t, HandleUpdateInvoice, http.MethodPut, "/api/invoices/321", saleFormWithLineDiscounts(t, saleIdentityAttempt), map[string]string{"id": "321"}, cookie)

	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%q, want 200", w.Code, w.Body.String())
	}
	bodies := writes.all()
	if len(bodies) != 1 {
		t.Fatalf("backend got %d bill writes, want 1", len(bodies))
	}
	assertLineDiscount(t, bodies[0], "products", 0, "5")
	assertLineDiscount(t, bodies[0], "manual_products", 0, "1.5")
}

// quickDraftBackend records what the quick draft invoice sends to the fake backend.
type quickDraftBackend struct {
	bills      *billWrites
	mu         sync.Mutex
	storeReads int
}

func (b *quickDraftBackend) reads() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.storeReads
}

// startQuickDraftBackend serves store 5 with branchID as its branch_id. A nil branchID means the store has no branch.
func startQuickDraftBackend(t *testing.T, branchID interface{}) *quickDraftBackend {
	t.Helper()
	helpers.APICache.Flush()
	t.Cleanup(helpers.APICache.Flush)

	backend := &quickDraftBackend{bills: &billWrites{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/stores/all":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"id":5,"name":"Main store"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/store/5":
			backend.mu.Lock()
			backend.storeReads++
			backend.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"detail": map[string]interface{}{"id": 5, "name": "Main store", "branch_id": branchID},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/bill":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode bill write: %v", err)
			}
			backend.bills.add(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("321"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	prevBackend := config.BackendDomain
	config.BackendDomain = server.URL
	t.Cleanup(func() { config.BackendDomain = prevBackend })
	return backend
}

func TestQuickDraftSendsStoreBranchAndCash(t *testing.T) {
	backend := startQuickDraftBackend(t, 7)
	cookie := seedSaleSession(t)

	w := postSaleForm(t, HandleCreateDraftInvoice, http.MethodPost, "/api/invoices/draft", "", nil, cookie)

	if w.Code >= http.StatusBadRequest {
		t.Errorf("draft status = %d body=%q, want success", w.Code, w.Body.String())
	}
	if got := backend.reads(); got != 1 {
		t.Errorf("store detail reads = %d, want 1", got)
	}
	bills := backend.bills.all()
	if len(bills) != 1 {
		t.Fatalf("backend got %d bill writes, want 1", len(bills))
	}
	if got := bills[0]["branch_id"]; got != float64(7) {
		t.Errorf("branch_id = %v, want 7 (the store's branch)", got)
	}
	if got := bills[0]["payment_method"]; got != float64(10) {
		t.Errorf("payment_method = %v, want 10 (cash)", got)
	}
}

// TestQuickDraftWithoutBranchSendsNoBill checks that a store without a branch stops the draft before any bill is sent.
// The spec gives no error wording, so the message must name the branch in English or Arabic.
func TestQuickDraftWithoutBranchSendsNoBill(t *testing.T) {
	backend := startQuickDraftBackend(t, nil)
	cookie := seedSaleSession(t)

	w := postSaleForm(t, HandleCreateDraftInvoice, http.MethodPost, "/api/invoices/draft", "", nil, cookie)

	if bills := backend.bills.all(); len(bills) != 0 {
		t.Errorf("backend got %d bill writes, want 0 when the store has no branch", len(bills))
	}
	if w.Code < http.StatusBadRequest {
		t.Errorf("status = %d, want an error status (>= 400) when the store has no branch", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(strings.ToLower(body), "branch") && !strings.Contains(body, "\u0641\u0631\u0639") {
		t.Errorf("error body = %q, want a message that names the branch", body)
	}
}
