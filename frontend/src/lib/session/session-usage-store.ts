import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import {
  isUsageEvent,
  usageCost,
  type SessionEventSource,
  type SessionUsage,
} from "./session-events"

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
    a.effort === b.effort &&
    a.costUsd === b.costUsd)

// createSessionUsageStore keeps the last reported context-window usage of every
// session, keyed by session id, fed by one subscription taken at creation —
// before any card mounts. get() answers null until the backend reports one, and
// the footer shows no context badge at all.
export function createSessionUsageStore(
  source: SessionEventSource,
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
      costUsd: usageCost(data),
    })
  })
  return store
}
