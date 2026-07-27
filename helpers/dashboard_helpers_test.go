package helpers

import (
	"testing"
	"time"
)

func TestParseFilterDateEndInclusive(t *testing.T) {
	end := ParseFilterDate("2026-02-14", true)
	if end == nil {
		t.Fatalf("expected end date")
	}
	if end.Hour() != 23 || end.Minute() != 59 {
		t.Fatalf("expected inclusive end-of-day, got %v", end)
	}
}

func TestParseFilterDateRFC3339(t *testing.T) {
	start := ParseFilterDate("2026-02-14T10:00:00Z", false)
	if start == nil {
		t.Fatalf("expected parsed RFC3339 date")
	}
	if start.UTC().Hour() != 10 {
		t.Fatalf("expected UTC hour 10, got %d", start.UTC().Hour())
	}
}

func TestResolveDashboardPeriod(t *testing.T) {
	now := time.Date(2026, time.May, 20, 14, 30, 0, 0, Riyadh)

	tests := []struct {
		name      string
		period    string
		wantStart string
		wantEnd   string
		wantValid bool
	}{
		{name: "today", period: "today", wantStart: "2026-05-20", wantEnd: "2026-05-20", wantValid: true},
		{name: "week", period: "week", wantStart: "2026-05-14", wantEnd: "2026-05-20", wantValid: true},
		{name: "month", period: "month", wantStart: "2026-05-01", wantEnd: "2026-05-20", wantValid: true},
		{name: "quarter", period: "quarter", wantStart: "2026-04-01", wantEnd: "2026-05-20", wantValid: true},
		{name: "year", period: "year", wantStart: "2026-01-01", wantEnd: "2026-05-20", wantValid: true},
		{name: "all", period: "all", wantValid: true},
		{name: "unknown", period: "invalid", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, valid := ResolveDashboardPeriod(tt.period, now)
			if start != tt.wantStart || end != tt.wantEnd || valid != tt.wantValid {
				t.Fatalf("ResolveDashboardPeriod(%q) = (%q, %q, %t), want (%q, %q, %t)",
					tt.period, start, end, valid, tt.wantStart, tt.wantEnd, tt.wantValid)
			}
		})
	}
}
