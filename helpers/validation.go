package helpers

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"
)

// FieldErrors maps field names to Arabic error messages.
// Passed to templates as "errors" so they can display inline messages.
type FieldErrors map[string]string

// HasErrors returns true if there are any validation errors.
func (fe FieldErrors) HasErrors() bool {
	return len(fe) > 0
}

// FieldRule describes a single validation rule for a form field.
type FieldRule struct {
	Field    string // form field name
	Value    string // the submitted value
	Required bool
	MinLen   int
	MaxLen   int
	Pattern  *regexp.Regexp // optional regex
	Email    bool           // validate as email
	// Arabic labels for user-facing messages
	Label      string
	PatternMsg string // custom pattern mismatch message
}

// Common Saudi patterns
var (
	PatternSaudiPhone = regexp.MustCompile(`^05\d{8}$`)
	PatternVATNumber  = regexp.MustCompile(`^\d{15}$`)
	PatternVIN        = regexp.MustCompile(`^[A-HJ-NPR-Z0-9]{17}$`)
)

// Validate runs all rules and returns field-level errors.
// Returns nil (not empty map) when there are no errors.
func Validate(rules []FieldRule) FieldErrors {
	errs := make(FieldErrors)

	for _, r := range rules {
		val := strings.TrimSpace(r.Value)

		// Required check
		if r.Required && val == "" {
			errs[r.Field] = fmt.Sprintf("%s مطلوب", r.Label)
			continue // skip further checks if empty
		}

		// Skip remaining checks if field is empty and not required
		if val == "" {
			continue
		}

		// MinLen
		if r.MinLen > 0 && utf8.RuneCountInString(val) < r.MinLen {
			errs[r.Field] = fmt.Sprintf("%s يجب أن يكون %d أحرف على الأقل", r.Label, r.MinLen)
			continue
		}

		// MaxLen
		if r.MaxLen > 0 && utf8.RuneCountInString(val) > r.MaxLen {
			errs[r.Field] = fmt.Sprintf("%s يجب ألا يتجاوز %d حرف", r.Label, r.MaxLen)
			continue
		}

		// Email
		if r.Email {
			if _, err := mail.ParseAddress(val); err != nil {
				errs[r.Field] = "البريد الإلكتروني غير صالح"
				continue
			}
		}

		// Pattern
		if r.Pattern != nil && !r.Pattern.MatchString(val) {
			msg := r.PatternMsg
			if msg == "" {
				msg = fmt.Sprintf("%s غير صالح", r.Label)
			}
			errs[r.Field] = msg
			continue
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// RenderFormWithErrors is a convenience that re-renders a form page with
// the submitted values and validation errors pre-filled.
// It merges errors and old values into the existing template data map.
func RenderFormWithErrors(data map[string]interface{}, errors FieldErrors, oldValues map[string]string) map[string]interface{} {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["errors"] = errors
	data["old"] = oldValues
	return data
}

// OldValues extracts form values from a parsed request for re-populating fields.
func OldValues(fields []string, formGetter func(string) string) map[string]string {
	old := make(map[string]string, len(fields))
	for _, f := range fields {
		old[f] = formGetter(f)
	}
	return old
}
