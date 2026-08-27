package helpers

import (
	"net/url"
	"testing"
)

func TestTypedListFilters_DropsUnknownKeys(t *testing.T) {
	q := url.Values{}
	q.Set("phone", "050")
	q.Set("vat_number", "300")
	q.Set("garbage", "x")
	got := TypedListFilters("clients", q)
	if got["phone"] != "050" || got["vat_number"] != "300" {
		t.Fatalf("missing accepted keys: %v", got)
	}
	if _, has := got["garbage"]; has {
		t.Fatalf("unknown key leaked: %v", got)
	}
}

func TestTypedListFiltersOrdersMatchesBackendFields(t *testing.T) {
	q := url.Values{
		"sequence_number": {"1001"},
		"phone":           {"050"},
		"vin":             {"VIN-123"},
	}

	got := TypedListFilters("orders", q)
	if len(got) != 2 || got["sequence_number"] != "1001" || got["phone"] != "050" {
		t.Fatalf("order filters = %v, want sequence_number and phone only", got)
	}
	if _, ok := got["vin"]; ok {
		t.Fatalf("unsupported order filter leaked: %v", got)
	}
}

func TestMatchTypedListFilters_PhoneStripsNonDigitsBothSides(t *testing.T) {
	row := map[string]string{"phone": "+966 50 123-4567"}
	if !MatchTypedListFilters(map[string]string{"phone": "050"}, row) {
		t.Fatalf("expected +966 50 ... to match prefix 050")
	}
	if !MatchTypedListFilters(map[string]string{"phone": "9665"}, row) {
		t.Fatalf("expected leading-9665 to match")
	}
	if MatchTypedListFilters(map[string]string{"phone": "0599"}, row) {
		t.Fatalf("0599 should NOT match")
	}
}

func TestMatchTypedListFilters_PhoneArabicIndic(t *testing.T) {
	row := map[string]string{"phone": "0501234567"}
	if !MatchTypedListFilters(map[string]string{"phone": "٠٥٠"}, row) {
		t.Fatalf("Arabic-Indic prefix should fold to 050")
	}
}

func TestMatchTypedListFilters_PhoneShortQueryIgnored(t *testing.T) {
	row := map[string]string{"phone": "0501234567"}
	// 3 digits → ignored (too noisy), so the filter is effectively empty
	// and ANY row passes.
	if !MatchTypedListFilters(map[string]string{"phone": "050"}, row) {
		t.Fatalf("050 (3 digits) should match because filter is min-4-digits")
	}
	// "abc" → 0 digits → ignored
	if !MatchTypedListFilters(map[string]string{"phone": "abc"}, row) {
		t.Fatalf("non-digit phone should be ignored, not block")
	}
}

func TestMatchTypedListFilters_VatNumberPrefix(t *testing.T) {
	row := map[string]string{"vat_number": "300123456700003"}
	if !MatchTypedListFilters(map[string]string{"vat_number": "300"}, row) {
		t.Fatalf("vat_number prefix should match")
	}
	if MatchTypedListFilters(map[string]string{"vat_number": "999"}, row) {
		t.Fatalf("vat_number 999 should NOT match")
	}
}

func TestMatchTypedListFilters_AllRequiredAreANDed(t *testing.T) {
	row := map[string]string{
		"phone":      "0501234567",
		"vat_number": "300123456700003",
	}
	if !MatchTypedListFilters(map[string]string{
		"phone":      "0501",
		"vat_number": "300",
	}, row) {
		t.Fatalf("AND match should pass when both prefixes hit")
	}
	if MatchTypedListFilters(map[string]string{
		"phone":      "0501",
		"vat_number": "999",
	}, row) {
		t.Fatalf("AND match should fail when one prefix misses")
	}
}

func TestMatchTypedListFilters_MissingFieldIsNonMatch(t *testing.T) {
	row := map[string]string{"phone": "0501234567"}
	if MatchTypedListFilters(map[string]string{"vat_number": "300"}, row) {
		t.Fatalf("missing vat_number on row must be non-match, not pass-through")
	}
}
