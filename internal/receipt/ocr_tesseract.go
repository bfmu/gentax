//go:build !nocgo

package receipt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/otiai10/gosseract/v2"
)

// TesseractClient implements OCRClient using the Tesseract OCR engine via gosseract.
// Requires CGO and Tesseract libraries to be installed on the host.
type TesseractClient struct{}

// NewTesseractClient returns a new TesseractClient.
func NewTesseractClient() *TesseractClient {
	return &TesseractClient{}
}

// ExtractData extracts DIAN receipt fields from image bytes using Tesseract.
// It writes a temporary file for gosseract (which requires a file path),
// extracts raw text, then parses fields with regex patterns.
func (c *TesseractClient) ExtractData(ctx context.Context, imageBytes []byte) (*OCRResult, error) {
	// Write image to a temp file because gosseract works with file paths.
	tmpFile, err := os.CreateTemp("", "receipt-*.png")
	if err != nil {
		return nil, fmt.Errorf("tesseract: create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(imageBytes); err != nil {
		return nil, fmt.Errorf("tesseract: write temp file: %w", err)
	}
	tmpFile.Close()

	client := gosseract.NewClient()
	defer client.Close()

	if err := client.SetImage(tmpFile.Name()); err != nil {
		return nil, fmt.Errorf("tesseract: set image: %w", err)
	}

	text, err := client.Text()
	if err != nil {
		return nil, fmt.Errorf("tesseract: extract text: %w", err)
	}

	result := parseOCRText(text)
	return result, nil
}

// Compiled regexes for DIAN field extraction.
var (
	// NIT: look for "NIT" label followed by digits, dots, hyphens.
	reNIT = regexp.MustCompile(`(?i)\bNIT[:\s#]*([0-9]{3,}[.\-0-9]*)`)

	// CUFE: 96-character hex string (DIAN electronic invoice unique code).
	reCUFE = regexp.MustCompile(`\b([0-9a-fA-F]{96})\b`)

	// Total: "TOTAL", "VALOR TOTAL", "TOTAL A PAGAR", "COMPRA NETA", "COMPRA", "SUBTOTAL", "VALOR" etc. followed by optional $ or COP and digits.
	reTotal = regexp.MustCompile(`(?i)\b(?:VALOR\s+)?(?:TOTAL(?:\s+(?:A\s+PAGAR|FACTURA|INCLUIDO|IVA|VENTA))?|COMPRA(?:\s+NETA)?|SUBTOTAL|VALOR)[^\n$0-9]*[\$COP\s]*([0-9][0-9.,]*)`)

	// Date: DD/MM/YYYY or YYYY-MM-DD.
	reDate = regexp.MustCompile(`\b(\d{2}/\d{2}/\d{4}|\d{4}-\d{2}-\d{2})\b`)
)

// normalizeColombianNumber converts a Colombian-formatted number string to a
// plain decimal string suitable for decimal.NewFromString.
//
// Colombian format rules:
//   - '.' is thousands separator (e.g. "12.500" = 12500)
//   - ',' is decimal separator  (e.g. "12.500,50" = 12500.50)
func normalizeColombianNumber(s string) string {
	hasDot := strings.ContainsRune(s, '.')
	hasComma := strings.ContainsRune(s, ',')

	switch {
	case hasDot && hasComma:
		// e.g. "1.234.567,89" → "1234567.89"
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	case hasDot:
		// e.g. "12.500" → "12500" (thousands separator only)
		s = strings.ReplaceAll(s, ".", "")
	case hasComma:
		// e.g. "12,500" — treat comma as decimal separator
		s = strings.ReplaceAll(s, ",", ".")
	}
	return s
}

// parseOCRText parses DIAN fields from raw Tesseract text output.
func parseOCRText(text string) *OCRResult {
	result := &OCRResult{}

	// NIT.
	if m := reNIT.FindStringSubmatch(text); len(m) > 1 {
		v := strings.TrimSpace(m[1])
		result.NIT = &v
	}

	// CUFE.
	if m := reCUFE.FindString(text); m != "" {
		result.CUFE = &m
	}

	// Total.
	if m := reTotal.FindStringSubmatch(text); len(m) > 1 {
		v := normalizeColombianNumber(strings.TrimSpace(m[1]))
		result.Total = &v
	}

	// Date.
	if m := reDate.FindString(text); m != "" {
		result.Date = &m
	}

	// Vendor: first non-empty, non-numeric line (usually the company name at top).
	lines := strings.Split(text, "\n")
	reOnlyDigitsOrPunct := regexp.MustCompile(`^[\d\s.,;:\-/\\*#@!%$]+$`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || reOnlyDigitsOrPunct.MatchString(line) {
			continue
		}
		result.Vendor = &line
		break
	}

	// Build raw JSON from full text and parsed fields.
	raw := map[string]interface{}{
		"raw_text": text,
		"fields": map[string]interface{}{
			"nit":    result.NIT,
			"cufe":   result.CUFE,
			"total":  result.Total,
			"date":   result.Date,
			"vendor": result.Vendor,
		},
	}
	rawJSON, _ := json.Marshal(raw)
	result.RawJSON = rawJSON

	return result
}
