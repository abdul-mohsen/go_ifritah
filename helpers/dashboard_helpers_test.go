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

func TestCurrentDashboardQuarter(t *testing.T) {
	now := time.Date(2026, time.May, 20, 14, 30, 0, 0, Riyadh)
	if got := CurrentDashboardQuarter(now); got != "2026-Q2" {
		t.Fatalf("CurrentDashboardQuarter() = %q, want %q", got, "2026-Q2")
	}
}

func TestDashboardQuarterChoices(t *testing.T) {
	choices := DashboardQuarterChoices()
	if len(choices) != 4 {
		t.Fatalf("DashboardQuarterChoices() returned %d options, want 4", len(choices))
	}
	for i, choice := range choices {
		if choice.Value != string(rune('1'+i)) {
			t.Errorf("choice %d value = %q, want %q", i, choice.Value, string(rune('1'+i)))
		}
		if choice.Label == "" {
			t.Errorf("choice %d has an empty label", i)
		}
	}
}

func TestDashboardYearOptions(t *testing.T) {
	options := DashboardYearOptions([]int{2024, 2026, 2025, 2026, 0, -1})
	if len(options) != 3 {
		t.Fatalf("DashboardYearOptions() returned %d options, want 3", len(options))
	}

	wantValues := []string{"2026", "2025", "2024"}
	for i, want := range wantValues {
		if options[i].Value != want {
			t.Errorf("option %d value = %q, want %q", i, options[i].Value, want)
		}
		if options[i].Label == "" {
			t.Errorf("option %d has an empty label", i)
		}
	}
}

func TestNormalizeDashboardYearSelection(t *testing.T) {
	now := time.Date(2026, time.May, 20, 14, 30, 0, 0, Riyadh)
	years := []int{2024, 2026}
	if got := NormalizeDashboardYearSelection("2024", years, now); got != "2024" {
		t.Fatalf("selected available year = %q, want 2024", got)
	}
	if got := NormalizeDashboardYearSelection("2025", years, now); got != "2026" {
		t.Fatalf("unavailable year = %q, want latest available year 2026", got)
	}
}

func TestCurrentDashboardQuarterSelection(t *testing.T) {
	now := time.Date(2026, time.May, 20, 14, 30, 0, 0, Riyadh)
	quarter, year := CurrentDashboardQuarterSelection(now)
	if quarter != "2" || year != "2026" {
		t.Fatalf("CurrentDashboardQuarterSelection() = (%q, %q), want (%q, %q)", quarter, year, "2", "2026")
	}
}

func TestResolveDashboardQuarter(t *testing.T) {
	now := time.Date(2026, time.May, 20, 14, 30, 0, 0, Riyadh)

	tests := []struct {
		name      string
		quarter   string
		wantStart string
		wantEnd   string
		wantValid bool
	}{
		{name: "current quarter", quarter: "2026-Q2", wantStart: "2026-04-01", wantEnd: "2026-05-20", wantValid: true},
		{name: "completed quarter", quarter: "2026-Q1", wantStart: "2026-01-01", wantEnd: "2026-03-31", wantValid: true},
		{name: "case insensitive", quarter: "2025-q4", wantStart: "2025-10-01", wantEnd: "2025-12-31", wantValid: true},
		{name: "invalid quarter", quarter: "2026-Q5", wantValid: false},
		{name: "future quarter", quarter: "2026-Q3", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, valid := ResolveDashboardQuarter(tt.quarter, now)
			if start != tt.wantStart || end != tt.wantEnd || valid != tt.wantValid {
				t.Fatalf("ResolveDashboardQuarter(%q) = (%q, %q, %t), want (%q, %q, %t)",
					tt.quarter, start, end, valid, tt.wantStart, tt.wantEnd, tt.wantValid)
			}
		})
	}
}

func TestResolveDashboardQuarterSelection(t *testing.T) {
	now := time.Date(2026, time.July, 28, 14, 30, 0, 0, Riyadh)

	start, end, quarter, year, valid := ResolveDashboardQuarterSelection("3", "2026", now)
	if !valid || start != "2026-07-01" || end != "2026-07-28" || quarter != "3" || year != "2026" {
		t.Fatalf("ResolveDashboardQuarterSelection() = (%q, %q, %q, %q, %t)",
			start, end, quarter, year, valid)
	}

	start, end, quarter, year, valid = ResolveDashboardQuarterSelection("2026-Q2", "", now)
	if !valid || start != "2026-04-01" || end != "2026-06-30" || quarter != "2" || year != "2026" {
		t.Fatalf("legacy quarter selection = (%q, %q, %q, %q, %t)",
			start, end, quarter, year, valid)
	}
}

func TestResolveDashboardYearSelection(t *testing.T) {
	now := time.Date(2026, time.May, 20, 14, 30, 0, 0, Riyadh)

	start, end, year, valid := ResolveDashboardYearSelection("2025", now)
	if !valid || start != "2025-01-01" || end != "2025-12-31" || year != "2025" {
		t.Fatalf("completed year = (%q, %q, %q, %t)", start, end, year, valid)
	}

	start, end, year, valid = ResolveDashboardYearSelection("2026", now)
	if !valid || start != "2026-01-01" || end != "2026-05-20" || year != "2026" {
		t.Fatalf("current year = (%q, %q, %q, %t)", start, end, year, valid)
	}

	if _, _, _, valid = ResolveDashboardYearSelection("2027", now); valid {
		t.Fatal("future year should be invalid")
	}
}
