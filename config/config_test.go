package config

import (
	"testing"
)

func TestFormatSAR_Zero(t *testing.T) {
	result := formatSAR(0)
	expected := "٠٫٠٠ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_Simple(t *testing.T) {
	result := formatSAR(123.45)
	expected := "١٢٣٫٤٥ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_Thousands(t *testing.T) {
	result := formatSAR(1234.50)
	expected := "١٬٢٣٤٫٥٠ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_LargeNumber(t *testing.T) {
	result := formatSAR(1234567.89)
	expected := "١٬٢٣٤٬٥٦٧٫٨٩ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_Negative(t *testing.T) {
	result := formatSAR(-500.00)
	expected := "-٥٠٠٫٠٠ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_WholeNumber(t *testing.T) {
	result := formatSAR(1000)
	expected := "١٬٠٠٠٫٠٠ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatSAR_SmallDecimal(t *testing.T) {
	result := formatSAR(0.99)
	expected := "٠٫٩٩ ر.س"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestToFloat64_Conversion(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{float64(1.5), 1.5},
		{float32(2.5), 2.5},
		{int(10), 10.0},
		{int64(20), 20.0},
		{int32(30), 30.0},
		{"not a number", 0.0},
		{nil, 0.0},
	}
	for _, tc := range tests {
		result := toFloat64(tc.input)
		if result != tc.expected {
			t.Errorf("toFloat64(%v) = %f, want %f", tc.input, result, tc.expected)
		}
	}
}
