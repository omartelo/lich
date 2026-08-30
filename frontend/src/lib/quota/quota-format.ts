import type { QuotaPlan, QuotaWindow } from "@/lib/api-types"

const HOUR_S = 60 * 60
const DAY_S = 24 * HOUR_S

// hottestWindow is the window a single readout has to show. The provider's own
// `is_active` verdict wins when it names one — that is the window actually
// binding the account, not a local guess — falling back to the window closest
// to spent (so a weekly cap about to run out does not hide behind a session
// window that just reset) when nothing is marked active, which is every
// payload shape that carries no such verdict at all. Ties in the fallback keep
// the first, the order the provider reported. Null for a plan with no windows.
export function hottestWindow(plan: QuotaPlan): QuotaWindow | null {
  const windows = plan.windows ?? []
  const active = windows.find((window) => window.active)
  if (active) {
    return active
  }
  let hottest: QuotaWindow | null = null
  for (const window of windows) {
    if (!hottest || window.percent > hottest.percent) {
      hottest = window
    }
  }
  return hottest
}

// shortWindow names a window's length in the width a status bar has: "5h",
// "wk", "30d". Empty when the provider did not report a length.
export function shortWindow(seconds: number): string {
  if (seconds <= 0) {
    return ""
  }
  if (seconds < DAY_S) {
    return `${Math.round(seconds / HOUR_S)}h`
  }
  const days = Math.round(seconds / DAY_S)
  return days === 7 ? "wk" : `${days}d`
}

// formatWindow names a window's length in full — "5h", "7d", "30d" — for the
// places that print it beside the window's own name.
export function formatWindow(seconds: number): string {
  if (seconds <= 0) {
    return ""
  }
  if (seconds < DAY_S) {
    return `${Math.round(seconds / HOUR_S)}h`
  }
  return `${Math.round(seconds / DAY_S)}d`
}

// timeLeft is how long until a window starts over, in the two coarsest units
// that apply ("6d 20h", "4h 12m", "41m"). Empty when there is no reset time, it
// is unparseable, or it has already passed — a countdown to a moment in the
// past is worse than no countdown.
export function timeLeft(resetsAt: string | undefined, now: Date): string {
  if (!resetsAt) {
    return ""
  }
  const reset = new Date(resetsAt).getTime()
  if (Number.isNaN(reset)) {
    return ""
  }
  const seconds = Math.floor((reset - now.getTime()) / 1000)
  if (seconds <= 0) {
    return ""
  }
  if (seconds >= DAY_S) {
    const days = Math.floor(seconds / DAY_S)
    return `${days}d ${Math.floor((seconds % DAY_S) / HOUR_S)}h`
  }
  if (seconds >= HOUR_S) {
    const hours = Math.floor(seconds / HOUR_S)
    return `${hours}h ${Math.floor((seconds % HOUR_S) / 60)}m`
  }
  return `${Math.max(1, Math.floor(seconds / 60))}m`
}
