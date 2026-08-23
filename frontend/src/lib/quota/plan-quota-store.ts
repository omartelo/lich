import type { QuotaPlan } from "@/lib/api-types"
import { Quota } from "@/lib/rpc"

export type PlanQuotaFetcher = (sessionId: string) => Promise<QuotaPlan[] | null>

// How often the reading is re-read while anything is watching it. The backend
// serves a five-minute cache, so this only decides how quickly a fresh reading
// reaches the screen — the providers are not asked any more often than that.
const REFRESH_MS = 60_000

// The snapshot a key nothing has read yet answers with. One shared array, so
// useSyncExternalStore sees the same reference on every render until a reading
// actually lands.
const NONE: QuotaPlan[] = []

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

// One poll loop and one reading per key. The key is the session the reading was
// taken for — "" is the machine-wide one the settings screen asks for — because
// a session spawned from a binary the user configured can spend another account
// entirely, and one card's numbers are not another's to show.
interface Entry {
  plans: QuotaPlan[]
  listeners: Set<() => void>
  timer?: ReturnType<typeof setTimeout>
  // Monotonic fetch id — a reading that started before the last subscriber left
  // must not publish after another one started.
  seq: number
}

// createPlanQuotaStore shares one poll loop per key across every subscriber to
// it. The fetcher is injected so the store is testable without React or the
// network, mirroring git-status-store.
export function createPlanQuotaStore(fetch: PlanQuotaFetcher, refreshMs = REFRESH_MS) {
  const entries = new Map<string, Entry>()

  const entryOf = (key: string): Entry => {
    let entry = entries.get(key)
    if (!entry) {
      entry = { plans: NONE, listeners: new Set(), seq: 0 }
      entries.set(key, entry)
    }
    return entry
  }

  const publish = (entry: Entry, next: QuotaPlan[]) => {
    if (unchanged(entry.plans, next)) {
      return
    }
    entry.plans = next
    for (const listener of entry.listeners) {
      listener()
    }
  }

  const refresh = (key: string, entry: Entry) => {
    const id = ++entry.seq
    void fetch(key)
      .then((next) => {
        if (id === entry.seq && next) {
          publish(entry, next)
        }
      })
      // A failed read keeps the previous numbers on screen: a gauge that blanks
      // on one dropped request is worse than one a minute out of date.
      .catch(() => {})
  }

  const poll = (key: string, entry: Entry) => {
    refresh(key, entry)
    entry.timer = setTimeout(() => poll(key, entry), refreshMs)
  }

  return {
    subscribe(key: string, listener: () => void): () => void {
      const entry = entryOf(key)
      entry.listeners.add(listener)
      if (entry.listeners.size === 1) {
        poll(key, entry)
      }
      return () => {
        entry.listeners.delete(listener)
        if (entry.listeners.size === 0) {
          clearTimeout(entry.timer)
          entry.timer = undefined
        }
      }
    },
    getSnapshot: (key: string): QuotaPlan[] => entries.get(key)?.plans ?? NONE,
  }
}

export const planQuotaStore = createPlanQuotaStore((sessionId) => Quota.Plans(sessionId))
