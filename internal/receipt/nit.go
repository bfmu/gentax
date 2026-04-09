package receipt

import (
	"strings"
	"unicode"
)

// dianWeights are applied right-to-left on the base digits (excluding check digit).
// Source: DIAN Resolución 000042 de 2020, Anexo Técnico.
var dianWeights = []int{3, 7, 13, 17, 19, 23, 29, 37, 41, 43}

// ValidateNIT validates a Colombian NIT (Número de Identificación Tributaria).
// Strips dots, hyphens, and spaces before validating.
// Accepts formats: "900455890-0", "9004558900", "900.455.890-0".
// Returns true if the check digit (last digit) matches the DIAN algorithm.
//
// DIAN algorithm:
//  1. Extract base digits (all but last) and check digit (last digit).
//  2. The base must be at least 8 digits (9+ total including check digit).
//  3. Apply weights right-to-left on base digits, accumulate sum.
//  4. remainder = sum mod 11
//     - if remainder < 2  → expected check digit = remainder
//     - else              → expected check digit = 11 - remainder
func ValidateNIT(raw string) bool {
	digits := stripNonDigits(raw)
	if len(digits) < 9 {
		return false
	}

	base := digits[:len(digits)-1]
	checkRune := rune(digits[len(digits)-1])
	checkDigit := int(checkRune - '0')

	// Apply weights to base digits right-to-left.
	sum := 0
	wi := 0 // weight index
	for i := len(base) - 1; i >= 0; i-- {
		if wi >= len(dianWeights) {
			break
		}
		d := int(base[i] - '0')
		sum += d * dianWeights[wi]
		wi++
	}

	remainder := sum % 11
	var expected int
	if remainder < 2 {
		expected = remainder
	} else {
		expected = 11 - remainder
	}

	return checkDigit == expected
}

// stripNonDigits removes all characters that are not ASCII digits.
func stripNonDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
