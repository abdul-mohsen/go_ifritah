package helpers

import (
	"strconv"
	"strings"
)

func ContainsInsensitive(value string, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(query))
}

// SanitizeForLog strips CR/LF from untrusted, request-derived values before
// they reach a log sink, preventing log forging/injection (e.g. a malicious
// username or cookie value spoofing extra fake log lines).
func SanitizeForLog(s string) string {
	return strings.NewReplacer("\n", " ", "\r", " ").Replace(s)
}

// SafeFloat extracts a float64 from an interface{} value (JSON numbers decode as float64).
func SafeFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	if f, ok := v.(float64); ok {
		return f
	}
	if s, ok := v.(string); ok {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	return 0
}

// SafeString extracts a string from an interface{} value, returns "" if nil or not a string.
func SafeString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
