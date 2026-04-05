package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// DefaultErrorMessage is the generic Arabic error shown when no specific message is available.
const DefaultErrorMessage = "حدث خطأ، يرجى المحاولة مرة أخرى"

// WriteErrorResponse writes an error response and triggers a frontend toast popup
// via the HX-Trigger header. It tries to extract a user-friendly message from
// backendResp. If no message is found, fallbackMsg is used. If fallbackMsg is
// also empty, a generic Arabic error message is shown.
func WriteErrorResponse(w http.ResponseWriter, statusCode int, backendResp *http.Response, fallbackMsg string) {
	msg := extractBackendMessage(backendResp)
	if msg == "" {
		msg = fallbackMsg
	}
	if msg == "" {
		msg = DefaultErrorMessage
	}

	triggerToast(w, msg, "error")
	w.WriteHeader(statusCode)
	// Write the message as body too (for non-HTMX consumers)
	_, _ = w.Write([]byte(msg))
}

// WriteSuccessToast sends a success toast notification via HX-Trigger.
func WriteSuccessToast(w http.ResponseWriter, msg string) {
	triggerToast(w, msg, "success")
}

// WriteSuccessRedirect sets a flash cookie + HX-Redirect.
// The flash cookie is read on the target page to show a success toast.
// This solves the problem of HX-Trigger not firing on HX-Redirect.
func WriteSuccessRedirect(w http.ResponseWriter, redirectURL string, msg string) {
	flash := map[string]string{"message": msg, "type": "success"}
	flashJSON, _ := json.Marshal(flash)
	http.SetCookie(w, &http.Cookie{
		Name:     "afrita_flash",
		Value:    url.QueryEscape(string(flashJSON)),
		Path:     "/",
		MaxAge:   10, // 10 seconds — read once then expire
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("HX-Redirect", redirectURL)
	w.WriteHeader(http.StatusOK)
}

// WriteErrorRedirect sets a flash error cookie + HX-Redirect.
// Unlike WriteErrorResponse (which returns a non-2xx status), this returns 200
// with HX-Redirect so HTMX reliably processes the redirect. The error shows
// as a toast on the target page via the flash cookie — same mechanism as success.
func WriteErrorRedirect(w http.ResponseWriter, redirectURL string, msg string) {
	if msg == "" {
		msg = DefaultErrorMessage
	}
	flash := map[string]string{"message": msg, "type": "error"}
	flashJSON, _ := json.Marshal(flash)
	http.SetCookie(w, &http.Cookie{
		Name:     "afrita_flash",
		Value:    url.QueryEscape(string(flashJSON)),
		Path:     "/",
		MaxAge:   10,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("HX-Redirect", redirectURL)
	w.WriteHeader(http.StatusOK)
}

// triggerToast sets the HX-Trigger header so the frontend shows a toast popup.
func triggerToast(w http.ResponseWriter, msg string, toastType string) {
	trigger := map[string]interface{}{
		"showToast": map[string]string{
			"message": msg,
			"type":    toastType,
		},
	}
	triggerJSON, err := json.Marshal(trigger)
	if err != nil {
		log.Printf("⚠️  Failed to marshal toast trigger: %v", err)
		return
	}
	w.Header().Set("HX-Trigger", EscapeNonASCII(string(triggerJSON)))
}

// extractBackendMessage tries to read a user-friendly message from a backend
// HTTP response. Delegates to ExtractMessageFromBytes for consistent parsing
// of string fields AND array validation errors.
func extractBackendMessage(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return ""
	}

	return ExtractMessageFromBytes(body)
}

// backendMsgTranslations maps common backend English error messages to Arabic.
// Keys are lowercased for case-insensitive matching.
var backendMsgTranslations = map[string]string{
	"not found":                    "العنصر غير موجود",
	"not authenticated":            "غير مصرح، يرجى تسجيل الدخول",
	"unauthorized":                 "غير مصرح، يرجى تسجيل الدخول",
	"forbidden":                    "ليس لديك صلاحية لهذا الإجراء",
	"internal server error":        "خطأ في الخادم، يرجى المحاولة لاحقاً",
	"bad request":                  "البيانات المرسلة غير صالحة",
	"validation error":             "خطأ في التحقق من البيانات",
	"method not allowed":           "الطريقة غير مسموحة",
	"conflict":                     "تعارض في البيانات، العنصر موجود بالفعل",
	"too many requests":            "طلبات كثيرة، يرجى الانتظار",
	"service unavailable":          "الخدمة غير متوفرة حالياً",
	"gateway timeout":              "انتهت مهلة الاتصال بالخادم",
	"connection refused":           "تعذر الاتصال بالخادم",
	"timeout":                      "انتهت مهلة الطلب",
	"invalid credentials":          "بيانات الدخول غير صحيحة",
	"invalid token":                "الجلسة منتهية، يرجى تسجيل الدخول مجدداً",
	"token expired":                "انتهت صلاحية الجلسة",
	"duplicate entry":              "هذا العنصر موجود بالفعل",
	"already exists":               "هذا العنصر موجود بالفعل",
	"record not found":             "السجل غير موجود",
	"no data found":                "لا توجد بيانات",
	"file too large":               "حجم الملف كبير جداً",
	"unsupported media type":       "نوع الملف غير مدعوم",
	"invalid email or password":    "البريد الإلكتروني أو كلمة المرور غير صحيحة",
	"email already registered":     "البريد الإلكتروني مسجل بالفعل",
	"permission denied":            "ليس لديك صلاحية لهذا الإجراء",
	"access denied":                "ليس لديك صلاحية لهذا الإجراء",
	"invalid input":                "البيانات المدخلة غير صالحة",
	"missing required field":       "يرجى تعبئة جميع الحقول المطلوبة",
	"field required":               "هذا الحقل مطلوب",
	"value is not a valid integer": "يجب إدخال رقم صحيح",
	"value is not a valid float":   "يجب إدخال رقم صحيح",
	"value is not a valid email":   "يرجى إدخال بريد إلكتروني صحيح",
}

// translateBackendMessage returns an Arabic translation for common English backend
// messages. Returns the original message if no translation is found.
func translateBackendMessage(msg string) string {
	lower := strings.ToLower(strings.TrimSpace(msg))
	if ar, ok := backendMsgTranslations[lower]; ok {
		return ar
	}
	// Check if the message contains a known English pattern
	for en, ar := range backendMsgTranslations {
		if strings.Contains(lower, en) {
			return ar
		}
	}
	return msg
}

// ExtractMessageFromBytes extracts a user-friendly message from raw response bytes.
// Checks JSON fields (detail, error, message, msg) first, then falls back to
// short plain-text bodies. Translates common English messages to Arabic.
// Returns empty string for HTML or empty input.
func ExtractMessageFromBytes(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// Try JSON with common error fields
	var jsonBody map[string]interface{}
	if json.Unmarshal(body, &jsonBody) == nil {
		for _, key := range []string{"detail", "error", "message", "msg"} {
			if v, ok := jsonBody[key]; ok {
				switch val := v.(type) {
				case string:
					if val != "" {
						return translateBackendMessage(val)
					}
				case []interface{}:
					// Array of validation errors: [{"msg":"..."},{"msg":"..."}]
					msgs := make([]string, 0, len(val))
					for _, item := range val {
						if m, ok := item.(map[string]interface{}); ok {
							if s, ok := m["msg"].(string); ok {
								msgs = append(msgs, translateBackendMessage(s))
							}
						}
					}
					if len(msgs) > 0 {
						return strings.Join(msgs, "\u060c ")
					}
				}
			}
		}
	}

	// If it's short plain-text (not HTML), use it — translate if English
	text := strings.TrimSpace(string(body))
	if len(text) > 0 && len(text) < 300 && !strings.Contains(text, "<") {
		return translateBackendMessage(text)
	}

	return ""
}

// WriteErrorResponseFromBytes writes an error response using raw response bytes.
// Extracts a message from body, falls back to fallbackMsg, then to DefaultErrorMessage.
func WriteErrorResponseFromBytes(w http.ResponseWriter, statusCode int, body []byte, fallbackMsg string) {
	msg := ExtractMessageFromBytes(body)
	if msg == "" {
		msg = fallbackMsg
	}
	if msg == "" {
		msg = DefaultErrorMessage
	}

	triggerToast(w, msg, "error")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(msg))
}

// EscapeNonASCII replaces non-ASCII runes with \uXXXX escape sequences.
// This ensures HTTP header values (like HX-Trigger) are ASCII-safe.
func EscapeNonASCII(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r > 127 {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
