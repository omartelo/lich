import type { CostMiss } from "./session-events"

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

// budgetShare is how much of a session's spend ceiling is gone, on the 0–100
// scale usageColor paints. It answers 0 — the scale's muted end — whenever
// there is no ceiling to be near, so a user who set none sees the readout
// exactly as it was before budgets existed.
//
// The share is capped at 100: past the ceiling there is no louder colour than
// red, and an uncapped number would only be read by a caller that does not
// exist. The figure it is taken against is API pricing from a table, so treat
// it as a nudge, not an invoice.
export function budgetShare(usd: number, budget: number): number {
  if (!Number.isFinite(usd) || !Number.isFinite(budget) || usd <= 0 || budget <= 0) {
    return 0
  }
  return Math.min((usd / budget) * 100, 100)
}

// COST_MISS_REASON is what the readout says in place of a figure it cannot
// produce. Each of these is a number that is not coming back on its own — the
// user would otherwise read the blank as zero spend — so the marker names it
// rather than leaving the corner empty.
//
// One sentence each, and it answers the only question the blank raises: did
// this cost nothing, or does lich not know? Why the total cannot be attributed
// and how rates are refreshed are lich explaining itself, and they lived here
// until a mockup showed the tooltip running three lines at footer size.
export const COST_MISS_REASON: Record<CostMiss, string> = {
  "mixed-models": "This conversation switched models, so lich cannot price it.",
  "unpriced-model": "No price for this model yet — lich needs the network to fetch one.",
}
