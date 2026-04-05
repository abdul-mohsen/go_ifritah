package helpers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestExtractMessageFromBytes_JSONDetail(t *testing.T) {
	body := []byte(`{"detail":"supplier_sequence_number is required"}`)
	msg := ExtractMessageFromBytes(body)
	if msg != "supplier_sequence_number is required" {
		t.Errorf("expected detail message, got: %q", msg)
	}
}

func TestExtractMessageFromBytes_JSONError(t *testing.T) {
	body := []byte(`{"error":"invalid store_id"}`)
	msg := ExtractMessageFromBytes(body)
	if msg != "invalid store_id" {
		t.Errorf("expected error message, got: %q", msg)
	}
}

func TestExtractMessageFromBytes_JSONMessage(t *testing.T) {
	body := []byte(`{"message":"not found"}`)
	msg := ExtractMessageFromBytes(body)
	// "not found" is translated to Arabic
	expected := "\u0627\u0644\u0639\u0646\u0635\u0631 \u063a\u064a\u0631 \u0645\u0648\u062c\u0648\u062f"
	if msg != expected {
		t.Errorf("expected %q, got: %q", expected, msg)
	}
}

func TestExtractMessageFromBytes_JSONDetailArray(t *testing.T) {
	body := []byte(`{"detail":[{"msg":"field required","type":"value_error"},{"msg":"invalid value","type":"type_error"}]}`)
	msg := ExtractMessageFromBytes(body)
	// "field required" is translated to Arabic
	expected := "\u0647\u0630\u0627 \u0627\u0644\u062d\u0642\u0644 \u0645\u0637\u0644\u0648\u0628\u060c invalid value"
	if msg != expected {
		t.Errorf("expected %q, got: %q", expected, msg)
	}
}

func TestExtractMessageFromBytes_PlainText(t *testing.T) {
	body := []byte("Bad Request")
	msg := ExtractMessageFromBytes(body)
	// "bad request" is translated to Arabic
	expected := "\u0627\u0644\u0628\u064a\u0627\u0646\u0627\u062a \u0627\u0644\u0645\u0631\u0633\u0644\u0629 \u063a\u064a\u0631 \u0635\u0627\u0644\u062d\u0629"
	if msg != expected {
		t.Errorf("expected %q, got: %q", expected, msg)
	}
}

func TestExtractMessageFromBytes_Empty(t *testing.T) {
	msg := ExtractMessageFromBytes(nil)
	if msg != "" {
		t.Errorf("expected empty, got: %q", msg)
	}
	msg = ExtractMessageFromBytes([]byte{})
	if msg != "" {
		t.Errorf("expected empty for empty slice, got: %q", msg)
	}
}

func TestExtractMessageFromBytes_HTML(t *testing.T) {
	body := []byte("<html><body>Error</body></html>")
	msg := ExtractMessageFromBytes(body)
	if msg != "" {
		t.Errorf("expected empty for HTML, got: %q", msg)
	}
}

func TestWriteErrorResponseFromBytes_UsesBackendMessage(t *testing.T) {
	w := httptest.NewRecorder()
	body := []byte(`{"detail":"store_id is required"}`)
	WriteErrorResponseFromBytes(w, 400, body, "fallback message")
	result := w.Body.String()
	if result != "store_id is required" {
		t.Errorf("expected backend message, got: %q", result)
	}
	// Check HX-Trigger header has toast
	hxTrigger := w.Header().Get("HX-Trigger")
	if hxTrigger == "" {
		t.Error("expected HX-Trigger header for toast")
	}
}

func TestTranslateBackendMessage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Not Found", "\u0627\u0644\u0639\u0646\u0635\u0631 \u063a\u064a\u0631 \u0645\u0648\u062c\u0648\u062f"},
		{"not found", "\u0627\u0644\u0639\u0646\u0635\u0631 \u063a\u064a\u0631 \u0645\u0648\u062c\u0648\u062f"},
		{"Unauthorized", "\u063a\u064a\u0631 \u0645\u0635\u0631\u062d\u060c \u064a\u0631\u062c\u0649 \u062a\u0633\u062c\u064a\u0644 \u0627\u0644\u062f\u062e\u0648\u0644"},
		{"Internal Server Error", "\u062e\u0637\u0623 \u0641\u064a \u0627\u0644\u062e\u0627\u062f\u0645\u060c \u064a\u0631\u062c\u0649 \u0627\u0644\u0645\u062d\u0627\u0648\u0644\u0629 \u0644\u0627\u062d\u0642\u0627\u064b"},
		// Unknown messages pass through unchanged
		{"custom_field_xyz is invalid", "custom_field_xyz is invalid"},
		// Arabic messages pass through unchanged
		{"\u062d\u062f\u062b \u062e\u0637\u0623", "\u062d\u062f\u062b \u062e\u0637\u0623"},
	}
	for _, tc := range tests {
		result := translateBackendMessage(tc.input)
		if result != tc.expected {
			t.Errorf("translateBackendMessage(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestWriteErrorResponseFromBytes_FallsBackToFallback(t *testing.T) {
	w := httptest.NewRecorder()
	WriteErrorResponseFromBytes(w, 400, []byte{}, "fallback message")
	result := w.Body.String()
	if result != "fallback message" {
		t.Errorf("expected fallback, got: %q", result)
	}
}

func TestWriteErrorResponseFromBytes_FallsBackToDefault(t *testing.T) {
	w := httptest.NewRecorder()
	WriteErrorResponseFromBytes(w, 400, []byte{}, "")
	result := w.Body.String()
	if result != DefaultErrorMessage {
		t.Errorf("expected default message, got: %q", result)
	}
}

// --- EscapeNonASCII tests ---

func TestEscapeNonASCII_PureASCII(t *testing.T) {
	input := `{"showToast":{"type":"error","message":"hello world"}}`
	result := EscapeNonASCII(input)
	if result != input {
		t.Errorf("pure ASCII should pass through unchanged, got: %q", result)
	}
}

func TestEscapeNonASCII_ArabicText(t *testing.T) {
	// Arabic "حدث خطأ" should be escaped to \uXXXX sequences
	input := "حدث خطأ"
	result := EscapeNonASCII(input)
	// Result must be pure ASCII
	for i, r := range result {
		if r > 127 {
			t.Errorf("non-ASCII rune %U at position %d in result: %q", r, i, result)
		}
	}
	// Result must contain \u escapes
	if len(result) <= len(input) {
		t.Errorf("escaped string should be longer than input, got len=%d", len(result))
	}
}

func TestEscapeNonASCII_MixedContent(t *testing.T) {
	input := `{"message":"حدث خطأ"}`
	result := EscapeNonASCII(input)
	// All ASCII structural parts should remain
	if result[0] != '{' || result[len(result)-1] != '}' {
		t.Errorf("JSON structure should be preserved, got: %q", result)
	}
	// Must be pure ASCII
	for _, r := range result {
		if r > 127 {
			t.Errorf("non-ASCII rune found in result: %q", result)
			break
		}
	}
}

func TestEscapeNonASCII_EmptyString(t *testing.T) {
	if EscapeNonASCII("") != "" {
		t.Error("empty string should return empty")
	}
}

func TestTriggerToast_ASCIISafeHeader(t *testing.T) {
	w := httptest.NewRecorder()
	// Use the Arabic default error message
	triggerToast(w, "حدث خطأ، يرجى المحاولة مرة أخرى", "error")

	hxTrigger := w.Header().Get("HX-Trigger")
	if hxTrigger == "" {
		t.Fatal("expected HX-Trigger header to be set")
	}

	// Every byte in the header must be ASCII (<=127)
	for i, r := range hxTrigger {
		if r > 127 {
			t.Errorf("non-ASCII rune %U at position %d in HX-Trigger: %q", r, i, hxTrigger)
			break
		}
	}

	// The header must be valid JSON that contains the original Arabic message
	// when parsed (JSON.parse handles \uXXXX natively)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(hxTrigger), &parsed); err != nil {
		t.Fatalf("HX-Trigger header is not valid JSON: %v\nValue: %s", err, hxTrigger)
	}
	toast, ok := parsed["showToast"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected showToast object, got: %v", parsed)
	}
	msg, ok := toast["message"].(string)
	if !ok || msg != "حدث خطأ، يرجى المحاولة مرة أخرى" {
		t.Errorf("expected Arabic message after JSON parse, got: %q", msg)
	}
}

func TestWriteErrorResponseFromBytes_ASCIISafeHeader(t *testing.T) {
	w := httptest.NewRecorder()
	// Trigger with Arabic fallback
	WriteErrorResponseFromBytes(w, 500, []byte{}, "")
	hxTrigger := w.Header().Get("HX-Trigger")
	for _, r := range hxTrigger {
		if r > 127 {
			t.Errorf("non-ASCII in HX-Trigger after WriteErrorResponseFromBytes: %q", hxTrigger)
			break
		}
	}
}
