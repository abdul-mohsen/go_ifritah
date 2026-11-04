package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"afrita/config"

	"github.com/gorilla/mux"
)

const (
	creditNoteTestPosID         = "pos-test-1"
	creditNoteTestRequestID     = "338b33a2-e22a-42d0-be14-1a6b2cbfcc71"
	creditNoteTestNextRequestID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
	creditNoteTestSourcePayload = `{"bill_id":42,"branch_id":7,"amount":"115.50","lines":[{"bill_product_id":101,"quantity":"2.00","price":"50.00","vat":"15.50"}]}`
)

func seedCreditNoteTestSession(t *testing.T) *http.Cookie {
	t.Helper()
	sessionID := "credit-note-test-" + strings.ReplaceAll(t.Name(), "/", "-")

	config.SessionTokensMutex.Lock()
	previousToken, hadToken := config.SessionTokens[sessionID]
	config.SessionTokens[sessionID] = "credit-note-test-token"
	config.SessionTokensMutex.Unlock()

	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		if hadToken {
			config.SessionTokens[sessionID] = previousToken
		} else {
			delete(config.SessionTokens, sessionID)
		}
		config.SessionTokensMutex.Unlock()
	})

	return &http.Cookie{Name: "session_id", Value: sessionID}
}

func setCreditNoteTestBackend(t *testing.T, handler http.Handler) {
	t.Helper()
	server := httptest.NewServer(handler)
	previousDomain := config.BackendDomain
	config.BackendDomain = server.URL
	t.Cleanup(func() {
		config.BackendDomain = previousDomain
		server.Close()
	})
}

func sendCreditNoteTestRequest(
	t *testing.T,
	handler http.HandlerFunc,
	method string,
	path string,
	values url.Values,
	vars map[string]string,
	cookie *http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if values != nil {
		body = strings.NewReader(values.Encode())
	}
	req := httptest.NewRequest(method, path, body)
	if values != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.AddCookie(cookie)
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}

	recorder := httptest.NewRecorder()
	handler(recorder, req)
	return recorder
}

func validCreditNoteTestForm() url.Values {
	return url.Values{
		"bill_id":         {"42"},
		"reason":          {"Damaged item"},
		"pos_id":          {creditNoteTestPosID},
		"bill_request_id": {creditNoteTestRequestID},
	}
}

func TestFetchCreditSourceParsesRealBackendDecimalStringShape(t *testing.T) {
	setCreditNoteTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/bill/42/credit-source" {
			t.Errorf("unexpected backend request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(creditNoteTestSourcePayload))
	}))

	source, failure := fetchCreditSource("credit-note-test-token", 42)
	if failure != nil {
		t.Fatalf("fetchCreditSource() failure = %+v", failure)
	}
	if source.Amount != "115.50" || len(source.Lines) != 1 {
		t.Fatalf("source = %+v, want amount 115.5 and one line", source)
	}
	line := source.Lines[0]
	if line.BillProductID != 101 || line.Quantity != "2.00" || line.Price != "50.00" || line.VAT != "15.50" {
		t.Errorf("source line = %+v, want product 101, quantity 2, price 50, vat 15.5", line)
	}
}

func TestCreditDecimalUnmarshalJSONAcceptsStringsAndNumbers(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  creditDecimal
	}{
		{name: "quoted decimal", value: `"115.50"`, want: "115.50"},
		{name: "JSON number", value: `115.50`, want: "115.50"},
		{name: "exponent number", value: `1.25e2`, want: "1.25e2"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var got creditDecimal
			if err := json.Unmarshal([]byte(testCase.value), &got); err != nil {
				t.Fatalf("unmarshal %s: %v", testCase.value, err)
			}
			if got != testCase.want {
				t.Errorf("value = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestCreditDecimalJSONRoundTripPreservesExactText(t *testing.T) {
	const exact = "9007199254740993.01"
	var got creditDecimal
	if err := json.Unmarshal([]byte(`"`+exact+`"`), &got); err != nil {
		t.Fatalf("unmarshal exact decimal: %v", err)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal exact decimal: %v", err)
	}
	if string(encoded) != `"`+exact+`"` {
		t.Fatalf("round-trip decimal = %s, want %q", encoded, exact)
	}
}

func TestHandleCreateCreditNotePreservesExactSourceDecimals(t *testing.T) {
	const exact = "9007199254740993.01"
	cookie := seedCreditNoteTestSession(t)
	var postedPayload []byte
	setCreditNoteTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/bill/42/credit-source":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"bill_id":42,"branch_id":7,"amount":"` + exact + `","lines":[{"bill_product_id":101,"quantity":"1.000","price":"` + exact + `","vat":"0.00"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/bill/credit":
			var err error
			postedPayload, err = io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read credit payload: %v", err)
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected backend request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))

	recorder := sendCreditNoteTestRequest(
		t,
		HandleCreateCreditNote,
		http.MethodPost,
		"/api/invoices/credit",
		validCreditNoteTestForm(),
		nil,
		cookie,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST status = %d, body = %q", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Amount json.RawMessage `json:"amount"`
		Lines  []struct {
			Quantity json.RawMessage `json:"quantity"`
			Price    json.RawMessage `json:"price"`
		} `json:"lines"`
	}
	if err := json.Unmarshal(postedPayload, &payload); err != nil {
		t.Fatalf("decode posted payload %q: %v", postedPayload, err)
	}
	if string(payload.Amount) != `"`+exact+`"` || len(payload.Lines) != 1 ||
		string(payload.Lines[0].Quantity) != `"1.000"` || string(payload.Lines[0].Price) != `"`+exact+`"` {
		t.Fatalf("posted decimal fields = %s, want exact source values", postedPayload)
	}
}

func TestCreditDecimalUnmarshalJSONRejectsInvalidTypesAndValues(t *testing.T) {
	for _, value := range []string{`null`, `true`, `{}`, `[]`, `"NaN"`, `"Inf"`, `"not-a-number"`, `1e999`} {
		t.Run(value, func(t *testing.T) {
			var got creditDecimal
			if err := json.Unmarshal([]byte(value), &got); err == nil {
				t.Errorf("unmarshal %s succeeded with %v, want an error", value, got)
			}
		})
	}
}

func TestHandleAddCreditNoteDisplaysVerifiedSource(t *testing.T) {
	cookie := seedCreditNoteTestSession(t)
	sourceRequests := 0
	setCreditNoteTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceRequests++
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/bill/42/credit-source" {
			t.Errorf("unexpected backend request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer credit-note-test-token" {
			t.Errorf("authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(creditNoteTestSourcePayload))
	}))

	recorder := sendCreditNoteTestRequest(
		t,
		HandleAddCreditNote,
		http.MethodGet,
		"/dashboard/invoices/credit/42",
		nil,
		map[string]string{"id": "42"},
		cookie,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	if sourceRequests != 1 {
		t.Fatalf("credit source requests = %d, want 1", sourceRequests)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		"115.50",
		"101",
		"50.00",
		`data-sale-scope="credit-42"`,
		`/static/js/sale-identity.js`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered credit form does not contain %q", want)
		}
	}
	for _, forbidden := range []string{`name="amount"`, `name="lines"`, `name="branch_id"`} {
		if strings.Contains(body, forbidden) {
			t.Errorf("rendered form contains untrusted source input %q", forbidden)
		}
	}
}

func TestHandleCreateCreditNoteUsesFreshSourceForNewRequestIdentity(t *testing.T) {
	cookie := seedCreditNoteTestSession(t)
	sourcePayloads := []string{
		`{"bill_id":42,"branch_id":7,"amount":"115.50","lines":[{"bill_product_id":101,"quantity":"2.00","price":"50.00","vat":"15.50"}]}`,
		`{"bill_id":42,"branch_id":9,"amount":"80.00","lines":[{"bill_product_id":202,"quantity":"1.00","price":"70.00","vat":"10.00"}]}`,
	}
	sourceRequests := 0
	createdPayloads := make([]creditNotePayload, 0, len(sourcePayloads))
	setCreditNoteTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/bill/42/credit-source":
			if sourceRequests >= len(sourcePayloads) {
				t.Errorf("unexpected extra source request")
				http.Error(w, "unexpected request", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(sourcePayloads[sourceRequests]))
			sourceRequests++
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/bill/credit":
			if got := r.Header.Get("Authorization"); got != "Bearer credit-note-test-token" {
				t.Errorf("authorization = %q, want bearer token", got)
			}
			var payload creditNotePayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode backend credit payload: %v", err)
				http.Error(w, "invalid payload", http.StatusBadRequest)
				return
			}
			createdPayloads = append(createdPayloads, payload)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected backend request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))

	form := validCreditNoteTestForm()
	form.Set("reason", "  Damaged item  ")
	form.Set("branch_id", "999")
	form.Set("amount", "999999")
	form.Set("lines", `[{"bill_product_id":999,"quantity":99,"price":99,"vat":99}]`)

	requestIDs := []string{creditNoteTestRequestID, creditNoteTestNextRequestID}
	for attempt := 0; attempt < 2; attempt++ {
		form := validCreditNoteTestForm()
		form.Set("bill_request_id", requestIDs[attempt])
		recorder := sendCreditNoteTestRequest(
			t,
			HandleCreateCreditNote,
			http.MethodPost,
			"/api/invoices/credit",
			form,
			nil,
			cookie,
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("POST attempt %d status = %d, body = %q", attempt+1, recorder.Code, recorder.Body.String())
		}
	}

	if sourceRequests != 2 {
		t.Fatalf("credit source requests = %d, want a fresh source for both POST attempts", sourceRequests)
	}
	if len(createdPayloads) != 2 {
		t.Fatalf("backend created %d credit notes, want 2 requests", len(createdPayloads))
	}

	wantAmounts := []creditDecimal{"115.50", "80.00"}
	wantBranchIDs := []int64{7, 9}
	wantProductIDs := []int64{101, 202}
	for i, payload := range createdPayloads {
		if payload.BillID != 42 || payload.BranchID != wantBranchIDs[i] {
			t.Errorf("attempt %d bill/branch = %d/%d, want 42/%d", i+1, payload.BillID, payload.BranchID, wantBranchIDs[i])
		}
		if payload.PosID != creditNoteTestPosID || payload.BillRequestID != requestIDs[i] {
			t.Errorf("attempt %d request identity = %q/%q, want %q/%q", i+1, payload.PosID, payload.BillRequestID, creditNoteTestPosID, requestIDs[i])
		}
		if payload.Reason != "Damaged item" {
			t.Errorf("attempt %d reason = %q, want trimmed reason", i+1, payload.Reason)
		}
		if payload.Amount != wantAmounts[i] || len(payload.Lines) != 1 ||
			payload.Lines[0].BillProductID != wantProductIDs[i] {
			t.Errorf("attempt %d payload source = amount %s, lines %+v; want amount %s and product %d", i+1, payload.Amount, payload.Lines, wantAmounts[i], wantProductIDs[i])
		}
	}
}

func TestHandleCreateCreditNoteReplaysPayloadAfterLostResponse(t *testing.T) {
	cookie := seedCreditNoteTestSession(t)
	sourceRequests := 0
	creditPosts := 0
	sourceExhausted := false
	var firstPayload []byte
	setCreditNoteTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/bill/42/credit-source":
			sourceRequests++
			if sourceExhausted {
				http.Error(w, "source exhausted", http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(creditNoteTestSourcePayload))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/bill/credit":
			creditPosts++
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read credit payload: %v", err)
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			if creditPosts == 1 {
				firstPayload = append([]byte(nil), payload...)
				sourceExhausted = true
				w.WriteHeader(http.StatusCreated)
				return
			}
			if !bytes.Equal(payload, firstPayload) {
				t.Errorf("retry payload = %s, want exact replay %s", payload, firstPayload)
				http.Error(w, "idempotency body mismatch", http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("idempotent replay"))
		default:
			t.Errorf("unexpected backend request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))

	form := validCreditNoteTestForm()
	firstResponse := sendCreditNoteTestRequest(
		t,
		HandleCreateCreditNote,
		http.MethodPost,
		"/api/invoices/credit",
		form,
		nil,
		cookie,
	)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first POST status = %d, body = %q", firstResponse.Code, firstResponse.Body.String())
	}
	// Discard the first success response to model it being lost by the browser.
	retryResponse := sendCreditNoteTestRequest(
		t,
		HandleCreateCreditNote,
		http.MethodPost,
		"/api/invoices/credit",
		form,
		nil,
		cookie,
	)
	if retryResponse.Code != http.StatusOK {
		t.Fatalf("retry POST status = %d, body = %q", retryResponse.Code, retryResponse.Body.String())
	}
	if sourceRequests != 1 {
		t.Errorf("source requests = %d, want 1; retry must skip exhausted source GET", sourceRequests)
	}
	if creditPosts != 2 {
		t.Errorf("credit POST requests = %d, want initial commit plus idempotent replay", creditPosts)
	}
}

func TestHandleCreateCreditNoteRejectsAttemptReusedForDifferentBill(t *testing.T) {
	cookie := seedCreditNoteTestSession(t)
	sourceRequests := 0
	creditPosts := 0
	setCreditNoteTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/bill/42/credit-source":
			sourceRequests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(creditNoteTestSourcePayload))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/bill/credit":
			creditPosts++
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected backend request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))

	first := sendCreditNoteTestRequest(
		t,
		HandleCreateCreditNote,
		http.MethodPost,
		"/api/invoices/credit",
		validCreditNoteTestForm(),
		nil,
		cookie,
	)
	if first.Code != http.StatusOK {
		t.Fatalf("first POST status = %d, body = %q", first.Code, first.Body.String())
	}

	reusedIdentity := validCreditNoteTestForm()
	reusedIdentity.Set("bill_id", "43")
	second := sendCreditNoteTestRequest(
		t,
		HandleCreateCreditNote,
		http.MethodPost,
		"/api/invoices/credit",
		reusedIdentity,
		nil,
		cookie,
	)
	if second.Code != http.StatusConflict {
		t.Fatalf("reused attempt status = %d, want 409; body = %q", second.Code, second.Body.String())
	}
	if sourceRequests != 1 || creditPosts != 1 {
		t.Errorf("backend requests = source %d, credit %d; want 1 source and 1 POST", sourceRequests, creditPosts)
	}
}

func TestHandleCreateCreditNoteRejectsInvalidInputBeforeFetchingSource(t *testing.T) {
	cases := []struct {
		name   string
		change func(url.Values)
	}{
		{name: "blank reason", change: func(form url.Values) { form.Set("reason", " \t ") }},
		{name: "missing POS", change: func(form url.Values) { form.Del("pos_id") }},
		{name: "invalid request ID", change: func(form url.Values) { form.Set("bill_request_id", "not-a-uuid") }},
		{name: "invalid bill ID", change: func(form url.Values) { form.Set("bill_id", "0") }},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cookie := seedCreditNoteTestSession(t)
			sourceRequests := 0
			setCreditNoteTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sourceRequests++
				http.Error(w, "source must not be fetched", http.StatusInternalServerError)
			}))

			form := validCreditNoteTestForm()
			testCase.change(form)
			recorder := sendCreditNoteTestRequest(
				t,
				HandleCreateCreditNote,
				http.MethodPost,
				"/api/invoices/credit",
				form,
				nil,
				cookie,
			)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("POST status = %d, want 400; body = %q", recorder.Code, recorder.Body.String())
			}
			if sourceRequests != 0 {
				t.Errorf("credit source requests = %d, want 0 for invalid input", sourceRequests)
			}
		})
	}
}

func TestHandleCreateCreditNoteFailsClosedOnUnverifiedSource(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		body       string
		wantStatus int
	}{
		{
			name:       "backend failure",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"error":"unavailable"}`,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "malformed response",
			statusCode: http.StatusOK,
			body:       `{"bill_id":`,
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "wrong bill",
			statusCode: http.StatusOK,
			body:       `{"bill_id":41,"branch_id":7,"amount":"1.00","lines":[{"bill_product_id":1,"quantity":"1.00","price":"1.00","vat":"0.00"}]}`,
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "empty remaining lines",
			statusCode: http.StatusOK,
			body:       `{"bill_id":42,"branch_id":7,"amount":"1.00","lines":[]}`,
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cookie := seedCreditNoteTestSession(t)
			creditCreates := 0
			setCreditNoteTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/api/v2/bill/credit" {
					creditCreates++
					w.WriteHeader(http.StatusCreated)
					return
				}
				if r.Method != http.MethodGet || r.URL.Path != "/api/v2/bill/42/credit-source" {
					t.Errorf("unexpected backend request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(testCase.statusCode)
				_, _ = w.Write([]byte(testCase.body))
			}))

			recorder := sendCreditNoteTestRequest(
				t,
				HandleCreateCreditNote,
				http.MethodPost,
				"/api/invoices/credit",
				validCreditNoteTestForm(),
				nil,
				cookie,
			)
			if recorder.Code != testCase.wantStatus {
				t.Errorf("POST status = %d, want %d; body = %q", recorder.Code, testCase.wantStatus, recorder.Body.String())
			}
			if creditCreates != 0 {
				t.Errorf("backend received %d credit-create requests for an unverified source", creditCreates)
			}
		})
	}
}
