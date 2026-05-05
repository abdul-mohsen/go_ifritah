package helpers

import (
	"strings"
	"time"
)

// Riyadh is the canonical location for the app. All wall-clock parsing,
// formatting and time.Now() calls anchor here.
var Riyadh *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Riyadh")
	if err != nil {
		loc = time.FixedZone("+03", 3*3600)
	}
	Riyadh = loc
	// Make every time.Now() and JSON marshal of time.Time emit Riyadh wall-clock
	// with +03:00 offset by default.
	time.Local = loc
}

// dateLayouts is the ordered list of input shapes parseDate accepts.
var dateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
}

// parseDate accepts any of the common date shapes the app sees (form input,
// backend response, legacy string fields) and returns a Riyadh-zoned time.
func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, l := range dateLayouts {
		if t, err := time.ParseInLocation(l, s, Riyadh); err == nil {
			return t.In(Riyadh), true
		}
	}
	return time.Time{}, false
}

// ToBackendDate converts any incoming date string into RFC3339 with the
// Riyadh offset, suitable for sending to the backend. Returns "" for empty
// or unparseable input.
//
// Use this for every outgoing date/datetime field on the wire.
func ToBackendDate(s string) string {
	t, ok := parseDate(s)
	if !ok {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ToBackendDatePtr is the *string variant of ToBackendDate for payload
// fields declared as *string with `omitempty`. Returns nil when the input
// is empty or unparseable.
func ToBackendDatePtr(s string) *string {
	out := ToBackendDate(s)
	if out == "" {
		return nil
	}
	return &out
}

// ToDisplayDate parses any backend date string and returns "YYYY-MM-DD"
// in Riyadh time. Returns "" on failure.
//
// Use this for every date shown to the user.
func ToDisplayDate(s string) string {
	t, ok := parseDate(s)
	if !ok {
		return ""
	}
	return t.Format("2006-01-02")
}

// ToDisplayDateTime parses any backend date string and returns
// "YYYY-MM-DD HH:MM" in Riyadh time. Returns "" on failure.
func ToDisplayDateTime(s string) string {
	t, ok := parseDate(s)
	if !ok {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}
