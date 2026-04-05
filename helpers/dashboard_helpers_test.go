package helpers

import "testing"

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
	if start.Hour() != 10 {
		t.Fatalf("expected hour 10, got %d", start.Hour())
	}
}
