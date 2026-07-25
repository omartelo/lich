import { isUsageEvent, type SessionUsage } from "./session-events"

// A subscription to the global usage event, injected so the store is testable
// without standing up the /events socket. Returns its unsubscribe.
export type UsageEventSource = (handler: (data: unknown) => void) => () => void

interface Entry {
  usage: SessionUsage | null
  listeners: Set<() => void>
}

// createSessionUsageStore keeps the last reported context-window usage of every
// session, keyed by session id, fed by one subscription taken at creation —
// before any card mounts. Same shape and reason as session-cwd-store: cards
// unmount with their project, so the value cannot live in them. The value object
// is only replaced on a real change, so its reference is stable for
// useSyncExternalStore between reports.
export function createSessionUsageStore(source: UsageEventSource) {
  const entries = new Map<string, Entry>()

  const entryOf = (id: string): Entry => {
    let entry = entries.get(id)
    if (!entry) {
      entry = { usage: null, listeners: new Set() }
      entries.set(id, entry)
    }
    return entry
  }

  source((data) => {
    if (!isUsageEvent(data)) {
      return
    }
    const entry = entryOf(data.id)
    // Bail on an unchanged report so subscribers skip the re-render, and the
    // snapshot reference stays put for useSyncExternalStore.
    if (
      entry.usage &&
      entry.usage.percent === data.percent &&
      entry.usage.tokens === data.tokens &&
      entry.usage.window === data.window &&
      entry.usage.model === data.model &&
      entry.usage.effort === data.effort
    ) {
      return
    }
    entry.usage = {
      percent: data.percent,
      tokens: data.tokens,
      window: data.window,
      model: data.model,
      effort: data.effort,
    }
    for (const listener of entry.listeners) {
      listener()
    }
  })

  const subscribe = (id: string, listener: () => void): (() => void) => {
    const entry = entryOf(id)
    entry.listeners.add(listener)
    return () => {
      entry.listeners.delete(listener)
    }
  }

  // get returns null while nothing has been reported — the card then shows no
  // context badge at all.
  const get = (id: string): SessionUsage | null => entries.get(id)?.usage ?? null

  return { subscribe, get }
}
