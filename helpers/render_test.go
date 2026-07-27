package helpers

import (
	"net/url"
	"testing"
)

func TestPaginationQueryPreservesFiltersWithoutPageState(t *testing.T) {
	values := url.Values{
		"q":               {"Acme & Sons"},
		"sequence_number": {"12345"},
		"state":           {"2"},
		"page":            {"3"},
		"per":             {"25"},
	}

	got := paginationQuery(values)
	want := "q=Acme+%26+Sons&sequence_number=12345&state=2"
	if got != want {
		t.Fatalf("paginationQuery() = %q, want %q", got, want)
	}
}
