package helpers

import "net/url"

// TypedListFilters returns the typed (per-field) filters that the smart-search
// frontend layer (static/js/smart-search.js) sends as separate query-string
// parameters for a given list resource. The keys mirror the FIELD_MENUS map in
// the JS, and the values come from the request URL.
//
// Resource keys: "invoices", "purchase_bills", "products", "clients",
// "suppliers", "orders", "cash_vouchers", "branches", "users".
//
// Returns a (param-name → value) map limited to non-empty values whose key is
// known for that resource. Caller passes this to MatchTypedListFilters along
// with the row's already-extracted field map. This keeps the BE wire contract
// happy — the BE may eventually receive the same params and short-circuit,
// but until then we narrow client-side here.
func TypedListFilters(resource string, q url.Values) map[string]string {
	allowed := typedFilterFields(resource)
	if len(allowed) == 0 {
		return nil
	}
	out := make(map[string]string, len(allowed))
	for _, name := range allowed {
		v := q.Get(name)
		if v == "" {
			continue
		}
		out[name] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// MatchTypedListFilters returns true when, for every (key, value) entry in
// filters, the row's field at the same key prefix-matches the value (after
// normalisation). An entry whose key is missing from row is treated as a
// non-match. Empty filters always match.
func MatchTypedListFilters(filters map[string]string, row map[string]string) bool {
	if len(filters) == 0 {
		return true
	}
	for k, v := range filters {
		// Phone is special: strip non-digits on both sides (matches BE
		// rule of REGEXP_REPLACE(col,'[^0-9]+','') generated column).
		// Skip when fewer than 4 digits — too noisy.
		if k == "phone" {
			needle := digitsOnly(v)
			if len(needle) < 4 {
				continue
			}
			hay := digitsOnly(row[k])
			if hay == "" || !startsWith(hay, needle) {
				return false
			}
			continue
		}
		needle := normalizeSearchText(v)
		if needle == "" {
			continue
		}
		hay := normalizeSearchText(row[k])
		if hay == "" {
			return false
		}
		// Prefix-match (per BE contract for ID-PREFIX fields), with
		// substring fallback for free-text-ish ones (vat_number,
		// commercial_registration may have leading whitespace etc.).
		if !startsWith(hay, needle) && !contains(hay, needle) {
			return false
		}
	}
	return true
}

// digitsOnly reduces s to ASCII digits, folding Arabic-Indic (٠-٩) and
// extended Arabic-Indic (۰-۹) digits to 0-9 first. Mirrors the BE's
// REGEXP_REPLACE(col,'[^0-9]+','') rule.
func digitsOnly(s string) string {
	if s == "" {
		return ""
	}
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			out = append(out, byte(r))
		case r >= 0x0660 && r <= 0x0669: // Arabic-Indic
			out = append(out, byte('0'+r-0x0660))
		case r >= 0x06F0 && r <= 0x06F9: // Extended Arabic-Indic
			out = append(out, byte('0'+r-0x06F0))
		}
	}
	return string(out)
}


func startsWith(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func typedFilterFields(resource string) []string {
	switch resource {
	case "invoices":
		return []string{"sequence_number", "phone", "vin"}
	case "purchase_bills":
		return []string{"sequence_number", "supplier_sequence_number"}
	case "products":
		return []string{"part_number", "barcode"}
	case "clients", "suppliers":
		return []string{"phone", "vat_number", "commercial_registration"}
	case "orders":
		return []string{"sequence_number", "phone", "vin"}
	case "cash_vouchers":
		return []string{"sequence_number"}
	case "branches":
		return []string{"phone"}
	case "users":
		return []string{"email", "phone"}
	}
	return nil
}
