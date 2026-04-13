/**
 * Formats a COP monetary amount string for display.
 *
 * - null / undefined → "—" (em dash, amount not yet extracted)
 * - "68000" → "68.000" (thousands separator, no decimals for whole numbers)
 * - "68000.50" → "68.000,50"
 */
export function formatAmount(amount: string | null | undefined): string {
  if (amount === null || amount === undefined) {
    return '—';
  }
  const num = parseFloat(amount);
  if (isNaN(num)) {
    return amount;
  }
  // Use COP locale formatting: thousands separator is ".", decimal is ","
  return num.toLocaleString('es-CO', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  });
}
