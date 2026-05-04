package helpers

import "testing"

func TestMatchSearchQuery_Empty(t *testing.T) {
	if !MatchSearchQuery("", "anything") {
		t.Error("empty query should match")
	}
	if !MatchSearchQuery("   ", "anything") {
		t.Error("whitespace-only query should match")
	}
}

func TestMatchSearchQuery_BasicSubstring(t *testing.T) {
	if !MatchSearchQuery("apple", "Green Apple Pie") {
		t.Error("expected case-insensitive substring match")
	}
	if MatchSearchQuery("apple", "Banana", "Cherry") {
		t.Error("expected no match")
	}
}

func TestMatchSearchQuery_ArabicAlefFolding(t *testing.T) {
	cases := []struct {
		q, field string
	}{
		{"احمد", "أحمد"},     // hamza-on-alef → bare alef
		{"احمد", "إحمد"},     // hamza-below
		{"احمد", "آحمد"},     // madda
		{"يوسف", "يوسف"},     // identity
		{"الى", "إلى"},       // alef-maqsura preserved on both sides
		{"عليه", "عَلِيهِ"}, // diacritics stripped
	}
	for _, c := range cases {
		if !MatchSearchQuery(c.q, c.field) {
			t.Errorf("%q should match %q", c.q, c.field)
		}
	}
}

func TestMatchSearchQuery_TaaMarbuta(t *testing.T) {
	if !MatchSearchQuery("شركه", "شركة الأفضل") {
		t.Error("ة should fold to ه")
	}
}

func TestMatchSearchQuery_AlefMaqsura(t *testing.T) {
	if !MatchSearchQuery("مستشفي", "مستشفى الملك") {
		t.Error("ى should fold to ي")
	}
}

func TestMatchSearchQuery_IndicDigits(t *testing.T) {
	if !MatchSearchQuery("123", "رقم ١٢٣") {
		t.Error("indic digits ١٢٣ should match 123")
	}
	if !MatchSearchQuery("١٢٣", "ID 123 Total") {
		t.Error("query in indic digits should match ASCII 123")
	}
}

func TestMatchSearchQuery_MultipleTokensAllMustMatch(t *testing.T) {
	// All tokens must match across the union of fields (AND semantics).
	if !MatchSearchQuery("ahmad ali", "Mr Ali", "Ahmad Khan") {
		t.Error("each token can match a different field")
	}
	if MatchSearchQuery("ahmad zzz", "Ali Ahmad") {
		t.Error("missing token must fail the match")
	}
}

func TestMatchSearchQuery_NoFieldsIsNoMatch(t *testing.T) {
	if MatchSearchQuery("x") {
		t.Error("no fields should not match a non-empty query")
	}
	if MatchSearchQuery("x", "", "") {
		t.Error("only-empty fields should not match")
	}
}

func TestMatchSearchQuery_TatweelStripped(t *testing.T) {
	if !MatchSearchQuery("سلام", "سـلـام") {
		t.Error("tatweel should be ignored")
	}
}

func TestMatchSearchQuery_PhoneSubstring(t *testing.T) {
	if !MatchSearchQuery("0501234567", "+966 0501234567") {
		t.Error("phone substring should match")
	}
	if !MatchSearchQuery("12345", "0501234567") {
		t.Error("partial phone should match")
	}
}

func TestNormalizeSearchText_CollapsesWhitespace(t *testing.T) {
	if got := normalizeSearchText("  Hello\t\nWorld   "); got != "hello world" {
		t.Errorf("got %q", got)
	}
}
