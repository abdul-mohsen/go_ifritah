package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testGzipEncoding   = "gzip"
	testCSRFCookieName = "csrf_token"
)

// ──────────────────────────────────────────────
// Recovery Middleware Tests
// ──────────────────────────────────────────────

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("expected 'ok', got '%s'", rr.Body.String())
	}
}

func TestRecoveryMiddleware_CatchesPanic(t *testing.T) {
	handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/crash", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "500") {
		t.Errorf("expected error page with 500, got '%s'", rr.Body.String())
	}
}

// ──────────────────────────────────────────────
// Request ID Middleware Tests
// ──────────────────────────────────────────────

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			t.Error("expected X-Request-ID to be set on request")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID in response headers")
	}
}

func TestRequestIDMiddleware_PreservesExistingID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id != "my-custom-id" {
			t.Errorf("expected 'my-custom-id', got '%s'", id)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "my-custom-id")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") != "my-custom-id" {
		t.Errorf("expected 'my-custom-id' in response, got '%s'", rr.Header().Get("X-Request-ID"))
	}
}

// ──────────────────────────────────────────────
// Logging Middleware Tests
// ──────────────────────────────────────────────

func TestLoggingMiddleware_SkipsStatic(t *testing.T) {
	called := false
	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/static/css/style.css", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should still be called for static assets")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestLoggingMiddleware_LogsDashboard(t *testing.T) {
	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("dashboard"))
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// ──────────────────────────────────────────────
// Security Headers Middleware Tests
// ──────────────────────────────────────────────

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for key, expected := range headers {
		got := rr.Header().Get(key)
		if got != expected {
			t.Errorf("header %s: expected '%s', got '%s'", key, expected, got)
		}
	}

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected Content-Security-Policy header to be set")
	}
}

// ──────────────────────────────────────────────
// Rate Limiting Middleware Tests
// ──────────────────────────────────────────────

func TestRateLimitMiddleware_AllowsNormalTraffic(t *testing.T) {
	handler := RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for normal traffic, got %d", rr.Code)
	}
}

func TestRateLimitMiddleware_BlocksExcessiveLogin(t *testing.T) {
	handler := RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Use unique IP so other tests don't interfere
	testIP := "192.168.99.99:12345"

	// First 5 should pass
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/login", nil)
		req.RemoteAddr = testIP
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}

	// 6th should be rate limited
	req := httptest.NewRequest("POST", "/login", nil)
	req.RemoteAddr = testIP
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after 5 login attempts, got %d", rr.Code)
	}
}

// ──────────────────────────────────────────────
// Gzip Middleware Tests
// ──────────────────────────────────────────────

func TestGzipMiddleware_CompressesWhenAccepted(t *testing.T) {
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Encoding") != testGzipEncoding {
		t.Error("expected gzip Content-Encoding")
	}
}

func TestGzipMiddleware_SkipsWhenNotAccepted(t *testing.T) {
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Encoding") == testGzipEncoding {
		t.Error("should not gzip when client doesn't accept")
	}
}

func TestGzipMiddleware_SkipsPDF(t *testing.T) {
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pdf content"))
	}))

	req := httptest.NewRequest("GET", "/bill_pdf/123", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Encoding") == testGzipEncoding {
		t.Error("should not gzip PDF paths")
	}
}

// ─── CSRF Middleware ─────────────────────────────────────────────────────────

func TestCSRFMiddleware_SetsCookie(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	resp := rr.Result()
	defer resp.Body.Close()
	cookies := resp.Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == testCSRFCookieName && len(c.Value) == 32 {
			found = true
		}
	}
	if !found {
		t.Error("CSRF middleware should set a 32-char csrf_token cookie on GET")
	}
}

func TestCSRFMiddleware_BlocksPOSTWithoutToken(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/dashboard/products/create", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("POST without CSRF token should be blocked, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_AllowsPOSTWithValidToken(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	// First GET to get the token
	getReq := httptest.NewRequest("GET", "/dashboard", nil)
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)

	var csrfToken string
	getResp := getRR.Result()
	defer getResp.Body.Close()
	for _, c := range getResp.Cookies() {
		if c.Name == testCSRFCookieName {
			csrfToken = c.Value
			break
		}
	}

	// POST with valid token
	postReq := httptest.NewRequest("POST", "/dashboard/products/create", nil)
	postReq.AddCookie(&http.Cookie{Name: testCSRFCookieName, Value: csrfToken})
	postReq.Header.Set("X-CSRF-Token", csrfToken)
	postRR := httptest.NewRecorder()
	handler.ServeHTTP(postRR, postReq)

	if postRR.Code != http.StatusOK {
		t.Errorf("POST with valid CSRF token should pass, got %d", postRR.Code)
	}
}

func TestCSRFMiddleware_BlocksPOSTWithWrongToken(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/dashboard/products/create", nil)
	req.AddCookie(&http.Cookie{Name: testCSRFCookieName, Value: "real-token-12345678901234567890"})
	req.Header.Set("X-CSRF-Token", "wrong-token-12345678901234567890")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("POST with wrong CSRF token should be blocked, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_ExemptLoginPath(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("POST", "/login", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Login POST should be CSRF-exempt, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_AllowsDELETEWithValidToken(t *testing.T) {
	handler := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Get token
	getReq := httptest.NewRequest("GET", "/dashboard", nil)
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)

	var csrfToken string
	getResp := getRR.Result()
	defer getResp.Body.Close()
	for _, c := range getResp.Cookies() {
		if c.Name == testCSRFCookieName {
			csrfToken = c.Value
			break
		}
	}

	// DELETE with valid token
	delReq := httptest.NewRequest("DELETE", "/api/invoices/1", nil)
	delReq.AddCookie(&http.Cookie{Name: testCSRFCookieName, Value: csrfToken})
	delReq.Header.Set("X-CSRF-Token", csrfToken)
	delRR := httptest.NewRecorder()
	handler.ServeHTTP(delRR, delReq)

	if delRR.Code != http.StatusOK {
		t.Errorf("DELETE with valid CSRF token should pass, got %d", delRR.Code)
	}
}

// ──────────────────────────────────────────────
// Body Size Limit Middleware Tests
// ──────────────────────────────────────────────

func TestBodySizeLimitMiddleware_RejectsLargeContentLength(t *testing.T) {
	handler := BodySizeLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/test", strings.NewReader("small"))
	req.ContentLength = 11 << 20 // 11MB — exceeds 10MB limit
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "413") {
		t.Errorf("expected JSON error with code 413, got %s", rr.Body.String())
	}
}

func TestBodySizeLimitMiddleware_AllowsNormalBody(t *testing.T) {
	handler := BodySizeLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	body := strings.Repeat("x", 1024) // 1KB — well within limit
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(body))
	req.ContentLength = int64(len(body))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestBodySizeLimitMiddleware_GETPassesThrough(t *testing.T) {
	handler := BodySizeLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for GET, got %d", rr.Code)
	}
}
