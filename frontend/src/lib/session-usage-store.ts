import { createKeyedStore, type ReadableKeyedStore } from "./keyed-store"
import { isUsageEvent, type SessionUsage } from "./session-events"

// A subscription to the global usage event, injected so the store is testable
// without standing up the /events socket. Returns its unsubscribe.
export type UsageEventSource = (handler: (data: unknown) => void) => () => void

// Every report rebuilds the object, so identity cannot decide whether anything
// moved — compare the fields, and an unchanged report keeps the old reference
// for useSyncExternalStore.
const sameUsage = (a: SessionUsage | null, b: SessionUsage | null): boolean =>
  a === b ||
  (a !== null &&
    b !== null &&
    a.percent === b.percent &&
    a.tokens === b.tokens &&
    a.window === b.window &&
    a.model === b.model &&
    a.effort === b.effort)

// createSessionUsageStore keeps the last reported context-window usage of every
// session, keyed by session id, fed by one subscription taken at creation —
// before any card mounts. get() answers null until the backend reports one, and
// the footer shows no context badge at all.
export function createSessionUsageStore(
  source: UsageEventSource,
): ReadableKeyedStore<SessionUsage | null> {
  const store = createKeyedStore<SessionUsage | null>(null, sameUsage)
  source((data) => {
    if (!isUsageEvent(data)) {
      return
    }
    store.set(data.id, {
      percent: data.percent,
      tokens: data.tokens,
      window: data.window,
      model: data.model,
      effort: data.effort,
    })
  })
  return store
}
