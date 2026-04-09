package receipt_test

import (
	"testing"

	"github.com/bmunoz/gentax/internal/receipt"
)

// TestValidateNIT_Valid tests known valid Colombian NITs using the DIAN check-digit algorithm.
func TestValidateNIT_Valid(t *testing.T) {
	cases := []struct {
		name string
		nit  string
	}{
		// base=900455890 → check digit 6 (computed with DIAN weights)
		{"plain digits 9004558906", "9004558906"},
		{"formatted with dots and hyphen", "900.455.890-6"},
		// base=800754951 → check digit 7
		{"plain digits 8007549517", "8007549517"},
		{"formatted 800.754.951-7", "800.754.951-7"},
		// base=123456789 → check digit 6
		{"plain digits 1234567896", "1234567896"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !receipt.ValidateNIT(tc.nit) {
				t.Errorf("ValidateNIT(%q) = false, want true", tc.nit)
			}
		})
	}
}

// TestValidateNIT_InvalidCheckDigit tests that a wrong last digit returns false.
func TestValidateNIT_InvalidCheckDigit(t *testing.T) {
	cases := []struct {
		name string
		nit  string
	}{
		// 9004558907 — correct is 6, changed to 7
		{"9004558906 with wrong digit 7", "9004558907"},
		// 9004558900 — correct is 6, changed to 0
		{"9004558906 with wrong digit 0", "9004558900"},
		// 8007549511 — correct is 7, changed to 1
		{"8007549517 with wrong digit 1", "8007549511"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if receipt.ValidateNIT(tc.nit) {
				t.Errorf("ValidateNIT(%q) = true, want false", tc.nit)
			}
		})
	}
}

// TestValidateNIT_TooShort tests that fewer than 9 digits returns false.
func TestValidateNIT_TooShort(t *testing.T) {
	cases := []struct {
		name string
		nit  string
	}{
		{"8 digits", "12345678"},
		{"7 digits", "1234567"},
		{"empty string", ""},
		{"only hyphens and dots", "---..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if receipt.ValidateNIT(tc.nit) {
				t.Errorf("ValidateNIT(%q) = true, want false", tc.nit)
			}
		})
	}
}

// TestValidateNIT_WithHyphensAndDots verifies that formatting is stripped before validation.
func TestValidateNIT_WithHyphensAndDots(t *testing.T) {
	// All three represent the same NIT 9004558906; all should produce identical results.
	raw := "9004558906"
	dotted := "900.455.890-6"
	spaced := "900 455 890 6"

	wantRaw := receipt.ValidateNIT(raw) // true
	if got := receipt.ValidateNIT(dotted); got != wantRaw {
		t.Errorf("ValidateNIT(%q) = %v, want %v (same as %q)", dotted, got, wantRaw, raw)
	}
	if got := receipt.ValidateNIT(spaced); got != wantRaw {
		t.Errorf("ValidateNIT(%q) = %v, want %v (same as %q)", spaced, got, wantRaw, raw)
	}
}
