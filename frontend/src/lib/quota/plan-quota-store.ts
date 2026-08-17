import type { QuotaPlan } from "@/lib/api-types"
import { Quota } from "@/lib/rpc"

export type PlanQuotaFetcher = () => Promise<QuotaPlan[] | null>

// How often the reading is re-read while anything is watching it. The backend
// serves a five-minute cache, so this only decides how quickly a fresh reading
// reaches the screen — the providers are not asked any more often than that.
const REFRESH_MS = 60_000

// Two readings are the same when every window of every plan is. Keeping the
// previous array on an unchanged reading is what stops a footer and a settings
// screen from re-rendering once a minute at a quota nobody is spending.
const unchanged = (a: QuotaPlan[], b: QuotaPlan[]): boolean =>
  a.length === b.length &&
  a.every((plan, i) => {
    const other = b[i]
    const windows = plan.windows ?? []
    const otherWindows = other.windows ?? []
    return (
      plan.provider === other.provider &&
      plan.status === other.status &&
      plan.plan === other.plan &&
      windows.length === otherWindows.length &&
      windows.every(
        (w, j) =>
          w.label === otherWindows[j].label &&
          w.percent === otherWindows[j].percent &&
          w.seconds === otherWindows[j].seconds &&
          w.resetsAt === otherWindows[j].resetsAt,
      )
    )
  })

// createPlanQuotaStore shares one poll loop across every subscriber. The fetcher
// is injected so the store is testable without React or the network, mirroring
// git-status-store.
export function createPlanQuotaStore(fetch: PlanQuotaFetcher, refreshMs = REFRESH_MS) {
  let plans: QuotaPlan[] = []
  const listeners = new Set<() => void>()
  let timer: ReturnType<typeof setTimeout> | undefined
  // Monotonic fetch id — a reading that started before the last subscriber left
  // must not publish after another one started.
  let seq = 0

  const publish = (next: QuotaPlan[]) => {
    if (unchanged(plans, next)) {
      return
    }
    plans = next
    for (const listener of listeners) {
      listener()
    }
  }

  const refresh = () => {
    const id = ++seq
    void fetch()
      .then((next) => {
        if (id === seq && next) {
          publish(next)
        }
      })
      // A failed read keeps the previous numbers on screen: a gauge that blanks
      // on one dropped request is worse than one a minute out of date.
      .catch(() => {})
  }

  const poll = () => {
    refresh()
    timer = setTimeout(poll, refreshMs)
  }

  return {
    subscribe(listener: () => void): () => void {
      listeners.add(listener)
      if (listeners.size === 1) {
        poll()
      }
      return () => {
        listeners.delete(listener)
        if (listeners.size === 0) {
          clearTimeout(timer)
          timer = undefined
        }
      }
    },
    getSnapshot: (): QuotaPlan[] => plans,
    refresh,
  }
}

export const planQuotaStore = createPlanQuotaStore(() => Quota.Plans())
