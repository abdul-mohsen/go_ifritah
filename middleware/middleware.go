package middleware

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Panic Recovery Middleware
// ──────────────────────────────────────────────

// RecoveryMiddleware catches panics from downstream handlers, logs the stack
// trace, and returns a 500 error page instead of crashing the server.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				reqID := r.Header.Get("X-Request-ID")
				log.Printf("🔴 PANIC [%s] %s %s: %v\n%s", reqID, r.Method, r.URL.Path, err, stack)

				// Don't write if headers already sent
				if rw, ok := w.(*loggingResponseWriter); ok && rw.written {
					return
				}

				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, `<!DOCTYPE html><html dir="rtl" lang="ar"><head><meta charset="utf-8"><title>500</title></head><body style="font-family:system-ui;text-align:center;padding:4rem;background:#111;color:#fff"><h1 style="font-size:3rem">500</h1><p style="font-size:1.2rem;color:#999">حدث خطأ في الخادم — يرجى المحاولة لاحقاً</p><a href="/dashboard" style="color:#3b82f6">العودة للوحة التحكم</a></body></html>`)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ──────────────────────────────────────────────
// Request ID Middleware
// ──────────────────────────────────────────────

// RequestIDMiddleware injects a unique X-Request-ID header into every request
// and response for distributed tracing.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = fmt.Sprintf("%x", b)
		}
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

// ──────────────────────────────────────────────
// Request Logging Middleware
// ──────────────────────────────────────────────

// loggingResponseWriter captures status code and bytes written for logging.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bytes      int
	written    bool
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	if !lrw.written {
		lrw.statusCode = code
		lrw.written = true
		lrw.ResponseWriter.WriteHeader(code)
	}
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if !lrw.written {
		lrw.WriteHeader(http.StatusOK)
	}
	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytes += n
	return n, err
}

// LoggingMiddleware logs every request with method, path, status, duration, and IP.
// Skips static asset requests to reduce noise.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip logging static assets
		if strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		reqID := r.Header.Get("X-Request-ID")
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = strings.Split(forwarded, ",")[0]
		}

		emoji := "✅"
		if lrw.statusCode >= 400 {
			emoji = "⚠️"
		}
		if lrw.statusCode >= 500 {
			emoji = "🔴"
		}

		log.Printf("%s [%s] %s %s → %d (%s) %dB — %s",
			emoji, reqID, r.Method, r.URL.Path,
			lrw.statusCode, duration.Round(time.Millisecond), lrw.bytes, ip)
	})
}

// ──────────────────────────────────────────────
// Security Headers Middleware
// ──────────────────────────────────────────────

// SecurityHeadersMiddleware adds standard security headers to all responses.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// CSP: allow inline styles/scripts (needed by HTMX + Tailwind), images, fonts
		// frame-src blob: needed for PDF print via hidden iframe
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://unpkg.com https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data: https:; "+
				"frame-src 'self' blob:; "+
				"connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// ──────────────────────────────────────────────
// Rate Limiting Middleware
// ──────────────────────────────────────────────

type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
}

type visitor struct {
	tokens     float64
	lastSeen   time.Time
	maxTokens  float64
	refillRate float64 // tokens per second
}

var loginLimiter = &rateLimiter{visitors: make(map[string]*visitor)}
var apiLimiter = &rateLimiter{visitors: make(map[string]*visitor)}

func init() {
	// Cleanup stale visitors every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			cleanupVisitors(loginLimiter)
			cleanupVisitors(apiLimiter)
		}
	}()
}

func cleanupVisitors(rl *rateLimiter) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, v := range rl.visitors {
		if time.Since(v.lastSeen) > 10*time.Minute {
			delete(rl.visitors, ip)
		}
	}
}

func (rl *rateLimiter) allow(ip string, maxTokens, refillRate float64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{
			tokens:     maxTokens - 1,
			lastSeen:   time.Now(),
			maxTokens:  maxTokens,
			refillRate: refillRate,
		}
		return true
	}

	// Token bucket refill
	elapsed := time.Since(v.lastSeen).Seconds()
	v.tokens += elapsed * refillRate
	if v.tokens > maxTokens {
		v.tokens = maxTokens
	}
	v.lastSeen = time.Now()

	if v.tokens >= 1 {
		v.tokens--
		return true
	}
	return false
}

// RateLimitMiddleware applies rate limiting. Login routes: 5/min. API routes: 60/min.
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = strings.Split(forwarded, ",")[0]
		}

		path := r.URL.Path
		var allowed bool

		switch {
		case path == "/login" && r.Method == "POST":
			// Login: 5 attempts per minute (burst 5, refill ~0.083/sec)
			allowed = loginLimiter.allow(ip, 5, 5.0/60.0)
		case strings.HasPrefix(path, "/api/"):
			// API endpoints: 60 per minute (burst 60, refill 1/sec)
			allowed = apiLimiter.allow(ip, 60, 1.0)
		default:
			allowed = true
		}

		if !allowed {
			w.Header().Set("Retry-After", "60")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"تم تجاوز الحد المسموح من الطلبات — يرجى المحاولة لاحقاً","code":429}`)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ──────────────────────────────────────────────
// Gzip Compression Middleware
// ──────────────────────────────────────────────

// GzipMiddleware compresses HTTP responses using gzip.
// Skips compression for binary content types (PDFs, images, etc.)
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip gzip for paths that serve binary content
		path := r.URL.Path
		if strings.HasSuffix(path, ".pdf") ||
			strings.Contains(path, "/bill_pdf/") ||
			strings.Contains(path, "/credit_bill_pdf/") ||
			strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".jpg") ||
			strings.HasSuffix(path, ".gif") ||
			strings.HasSuffix(path, ".ico") ||
			strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".woff") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		// Remove Content-Length since gzip changes the size
		w.Header().Del("Content-Length")

		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		defer gz.Close()

		gw := gzipResponseWriter{ResponseWriter: w, Writer: gz}
		next.ServeHTTP(gw, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// csrfExemptPaths are public routes that don't need CSRF validation.
var csrfExemptPaths = map[string]bool{
	"/login":               true,
	"/api/register":        true,
	"/api/forgot-password": true,
	"/api/refresh":         true,
}

// ──────────────────────────────────────────────
// Request Body Size Limit Middleware
// ──────────────────────────────────────────────

// BodySizeLimitMiddleware restricts the maximum request body size.
// Prevents large payloads from consuming server memory (default 10MB).
func BodySizeLimitMiddleware(next http.Handler) http.Handler {
	const maxBodySize = 10 << 20 // 10 MB
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxBodySize {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			fmt.Fprint(w, `{"error":"حجم الطلب كبير جداً — الحد الأقصى 10 ميجابايت","code":413}`)
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		}
		next.ServeHTTP(w, r)
	})
}

// CSRFMiddleware implements the double-submit cookie pattern.
// It sets a csrf_token cookie on every response and validates that
// POST/PUT/DELETE requests include a matching X-CSRF-Token header.
// This works seamlessly with HTMX's hx-headers configuration.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate or read existing CSRF token from cookie
		var csrfToken string
		cookie, err := r.Cookie("csrf_token")
		if err != nil || cookie.Value == "" {
			// Generate new token
			b := make([]byte, 16)
			if _, err := rand.Read(b); err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			csrfToken = hex.EncodeToString(b)
		} else {
			csrfToken = cookie.Value
		}

		// Always set/refresh the cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "csrf_token",
			Value:    csrfToken,
			Path:     "/",
			HttpOnly: false, // JavaScript/HTMX needs to read this
			Secure:   false, // Set to true in production with HTTPS
			SameSite: http.SameSiteStrictMode,
			MaxAge:   86400, // 24 hours
		})

		// Only validate on state-changing methods
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH" {
			// Skip exempt paths
			if !csrfExemptPaths[r.URL.Path] {
				headerToken := r.Header.Get("X-CSRF-Token")
				if headerToken == "" {
					// Also check form field as fallback
					headerToken = r.FormValue("csrf_token")
				}

				if headerToken != csrfToken {
					log.Printf("⚠️  CSRF validation failed for %s %s (expected=%s, got=%s)",
						r.Method, r.URL.Path, csrfToken[:8]+"...", safePrefix(headerToken, 8))
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"error":"CSRF token validation failed - انتهت صلاحية الجلسة، يرجى تحديث الصفحة"}`))
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// safePrefix returns the first n chars of a string, or the full string if shorter.
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
