// What one provider's row in Settings says about itself without being opened.
// Both readings are derived from state the app already holds, so a row costs no
// request of its own: the sessions come from the projects store, the plan from
// the quota store.
import type { QuotaPlan } from "@/lib/api-types"
import { hottestWindow, formatWindow } from "@/lib/quota/quota-format"
import type { SessionState } from "@/lib/session/sessions"

/** How many open sessions across every project this provider is running. The
 * answer to "can I turn this off?", which is the question the switch beside it
 * raises. Counts the sessions on screen; a parked row is not one. */
export function countOpenSessions(state: SessionState, kind: string): number {
  let count = 0
  for (const project of Object.values(state)) {
    for (const session of project.sessions) {
      if (session.kind === kind) {
        count += 1
      }
    }
  }
  return count
}

/** The plan reading in the width a list row has: "62% of the 5h window". Empty
 * for a provider lich reads no plan from and while the first reading is in
 * flight, so the row falls back to saying something else.
 *
 * A signed-out reading is words rather than nothing: the row is where a login
 * that has lapsed becomes visible, and an empty string would read as "this
 * provider has no plan", which is a different fact. */
export function planSummary(plan: QuotaPlan | null): string {
  if (!plan) {
    return ""
  }
  if (plan.status === "signed-out") {
    return "Signed out"
  }
  if (plan.status !== "ok") {
    return ""
  }
  const window = hottestWindow(plan)
  if (!window) {
    return ""
  }
  const length = formatWindow(window.seconds)
  return length ? `${window.percent}% of the ${length} window` : `${window.percent}% used`
}
