package helpers

import (
	"testing"
)

func TestValidate_RequiredFieldMissing(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "name", Value: "", Required: true, Label: "الاسم"},
	})
	if errs == nil {
		t.Fatal("expected errors, got nil")
	}
	if _, ok := errs["name"]; !ok {
		t.Fatal("expected error for 'name' field")
	}
	if errs["name"] != "الاسم مطلوب" {
		t.Fatalf("unexpected message: %s", errs["name"])
	}
}

func TestValidate_RequiredFieldPresent(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "name", Value: "أحمد", Required: true, Label: "الاسم"},
	})
	if errs != nil {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_MinLen(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "name", Value: "أ", Required: true, MinLen: 2, Label: "الاسم"},
	})
	if errs == nil || errs["name"] == "" {
		t.Fatal("expected minLen error")
	}
}

func TestValidate_MaxLen(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "name", Value: "أحمد محمد علي", Required: true, MaxLen: 5, Label: "الاسم"},
	})
	if errs == nil || errs["name"] == "" {
		t.Fatal("expected maxLen error")
	}
}

func TestValidate_InvalidEmail(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "email", Value: "not-an-email", Email: true, Required: true, Label: "البريد"},
	})
	if errs == nil || errs["email"] == "" {
		t.Fatal("expected email error")
	}
}

func TestValidate_ValidEmail(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "email", Value: "test@example.com", Email: true, Required: true, Label: "البريد"},
	})
	if errs != nil {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_PatternMatch(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "phone", Value: "0512345678", Required: true, Pattern: PatternSaudiPhone, Label: "الهاتف", PatternMsg: "رقم غير صالح"},
	})
	if errs != nil {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_PatternMismatch(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "phone", Value: "123", Required: true, Pattern: PatternSaudiPhone, Label: "الهاتف", PatternMsg: "رقم غير صالح"},
	})
	if errs == nil || errs["phone"] != "رقم غير صالح" {
		t.Fatalf("expected pattern error, got: %v", errs)
	}
}

func TestValidate_OptionalEmptySkipped(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "phone", Value: "", Required: false, Pattern: PatternSaudiPhone, Label: "الهاتف"},
	})
	if errs != nil {
		t.Fatalf("expected no errors for optional empty field, got: %v", errs)
	}
}

func TestValidate_VATPattern(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "vat", Value: "300000000000003", Pattern: PatternVATNumber, Label: "الرقم الضريبي"},
	})
	if errs != nil {
		t.Fatalf("expected no errors for valid VAT, got: %v", errs)
	}

	errs = Validate([]FieldRule{
		{Field: "vat", Value: "12345", Pattern: PatternVATNumber, Label: "الرقم الضريبي", PatternMsg: "يجب أن يتكون من 15 رقم"},
	})
	if errs == nil || errs["vat"] == "" {
		t.Fatal("expected VAT pattern error")
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	errs := Validate([]FieldRule{
		{Field: "name", Value: "", Required: true, Label: "الاسم"},
		{Field: "email", Value: "bad", Required: true, Email: true, Label: "البريد"},
		{Field: "phone", Value: "123", Required: true, Pattern: PatternSaudiPhone, Label: "الهاتف", PatternMsg: "رقم غير صالح"},
	})
	if errs == nil {
		t.Fatal("expected 3 errors")
	}
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
	}
}

func TestFieldErrors_HasErrors(t *testing.T) {
	fe := FieldErrors{"name": "مطلوب"}
	if !fe.HasErrors() {
		t.Fatal("expected HasErrors to be true")
	}

	fe2 := FieldErrors{}
	if fe2.HasErrors() {
		t.Fatal("expected HasErrors to be false for empty map")
	}
}

func TestOldValues(t *testing.T) {
	getter := func(key string) string {
		m := map[string]string{"name": "أحمد", "email": "a@b.com"}
		return m[key]
	}
	old := OldValues([]string{"name", "email", "phone"}, getter)
	if old["name"] != "أحمد" {
		t.Fatalf("expected أحمد, got %s", old["name"])
	}
	if old["phone"] != "" {
		t.Fatalf("expected empty for missing field, got %s", old["phone"])
	}
}

func TestRenderFormWithErrors(t *testing.T) {
	data := map[string]interface{}{"title": "test"}
	errs := FieldErrors{"name": "مطلوب"}
	old := map[string]string{"name": "x"}

	result := RenderFormWithErrors(data, errs, old)
	if result["errors"] == nil {
		t.Fatal("expected errors in result")
	}
	if result["old"] == nil {
		t.Fatal("expected old in result")
	}
	if result["title"] != "test" {
		t.Fatal("original data should be preserved")
	}
}

func TestRenderFormWithErrors_NilData(t *testing.T) {
	result := RenderFormWithErrors(nil, FieldErrors{"x": "y"}, map[string]string{"x": "z"})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["errors"] == nil {
		t.Fatal("expected errors in result")
	}
}
