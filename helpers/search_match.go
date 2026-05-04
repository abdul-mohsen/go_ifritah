package helpers

import (
	"strings"
	"unicode"
)

// MatchSearchQuery returns true when every whitespace-separated token in
// `query` is found inside at least one of the provided `fields`, after the
// following normalization is applied to both the query and each field:
//
//   - Unicode NFKD decomposition (separates base letters from combining marks)
//   - Combining mark stripping (Arabic harakat, Latin diacritics)
//   - Arabic letter folding:  أ/إ/آ/ٱ → ا,  ى → ي,  ة → ه,  ؤ → و,  ئ → ي
//   - Indic-Arabic digits (٠-٩) → ASCII (0-9)
//   - Lower-cased
//   - Whitespace collapsed
//
// An empty query string matches everything (caller should usually short-circuit
// before calling, but this keeps templates safe).
//
// This is a deliberately small, dependency-light substitute for a real FTS
// engine, intended only as the FE's stop-gap until the backend lands its own
// `LIKE`/FULLTEXT search across the same fields.
func MatchSearchQuery(query string, fields ...string) bool {
	q := normalizeSearchText(query)
	if q == "" {
		return true
	}
	tokens := strings.Fields(q)
	if len(tokens) == 0 {
		return true
	}

	// Normalize each field once.
	normFields := make([]string, 0, len(fields))
	for _, f := range fields {
		if f == "" {
			continue
		}
		normFields = append(normFields, normalizeSearchText(f))
	}
	if len(normFields) == 0 {
		return false
	}

	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		hit := false
		for _, f := range normFields {
			if strings.Contains(f, tok) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// normalizeSearchText applies the folding rules documented on MatchSearchQuery.
func normalizeSearchText(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		// Strip Unicode combining marks (Mn = nonspacing mark; covers Arabic
		// harakat and Latin combining diacritics).
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		// Drop tatweel (Arabic letter elongator).
		if r == 'ـ' {
			continue
		}
		// Arabic letter folding.
		switch r {
		case 'أ', 'إ', 'آ', 'ٱ':
			r = 'ا'
		case 'ى':
			r = 'ي'
		case 'ة':
			r = 'ه'
		case 'ؤ':
			r = 'و'
		case 'ئ':
			r = 'ي'
		}
		// Indic-Arabic digits → ASCII.
		if r >= '\u0660' && r <= '\u0669' {
			r = '0' + (r - '\u0660')
		}
		// Eastern-Arabic (Persian) digits → ASCII.
		if r >= '\u06F0' && r <= '\u06F9' {
			r = '0' + (r - '\u06F0')
		}
		b.WriteRune(unicode.ToLower(r))
	}

	// Collapse runs of whitespace to a single space.
	out := strings.Join(strings.Fields(b.String()), " ")
	return out
}