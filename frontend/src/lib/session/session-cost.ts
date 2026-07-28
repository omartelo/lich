// formatCost renders a session's spend for the footer, where it sits beside the
// context percent at 12px and has to stay readable at a glance.
//
// The scale it spans is wide — a one-question session costs fractions of a
// cent, a long one runs to dollars — so the precision follows the number: cents
// once there are dollars to anchor them, three decimals below that (the
// difference between $0.004 and $0.02 is the whole signal early in a session),
// and a floor of "<$0.001" rather than a row of zeros, which would read as free.
export function formatCost(usd: number): string {
  if (!Number.isFinite(usd) || usd < 0) {
    return ""
  }
  if (usd === 0) {
    return "$0"
  }
  if (usd < 0.001) {
    return "<$0.001"
  }
  return usd >= 1 ? `$${usd.toFixed(2)}` : `$${usd.toFixed(3)}`
}
