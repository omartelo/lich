// The shape every per-session store shares: a value kept per key, with
// subscribers scoped to one key. Entries outlive their listeners on purpose —
// SessionSidebar only mounts the active project's cards, so a report arriving
// while a card is unmounted has to survive somewhere that is not the card.
//
// Read side only. A store hands this out; the event handler that feeds it keeps
// `set` to itself.
export interface ReadableKeyedStore<T> {
  subscribe(key: string, listener: () => void): () => void
  get(key: string): T
}

export interface KeyedStore<T> extends ReadableKeyedStore<T> {
  /** Publish a value. A value `equal` to the current one notifies nobody. */
  set(key: string, value: T): void
}

interface Entry<T> {
  value: T
  listeners: Set<() => void>
}

// createKeyedStore is the module-level store behind the per-session readouts
// (cwd, agent, usage): one shared map, subscribers per key, no state library.
//
// `empty` is what get() answers for a key nothing has reported yet, and the
// baseline every entry starts at. `equal` guards the notification: a report
// that changes nothing must not re-render its subscribers, which for the plain
// snapshots (a string, a union) is the default identity check. A store whose
// value is an object rebuilt on every report passes its own comparison, so the
// snapshot reference stays put for useSyncExternalStore.
export function createKeyedStore<T>(
  empty: T,
  equal: (a: T, b: T) => boolean = Object.is,
): KeyedStore<T> {
  const entries = new Map<string, Entry<T>>()

  const entryOf = (key: string): Entry<T> => {
    let entry = entries.get(key)
    if (!entry) {
      entry = { value: empty, listeners: new Set() }
      entries.set(key, entry)
    }
    return entry
  }

  return {
    set(key, value) {
      const entry = entryOf(key)
      if (equal(entry.value, value)) {
        return
      }
      entry.value = value
      for (const listener of entry.listeners) {
        listener()
      }
    },
    subscribe(key, listener) {
      const entry = entryOf(key)
      entry.listeners.add(listener)
      return () => {
        entry.listeners.delete(listener)
      }
    },
    get: (key) => entries.get(key)?.value ?? empty,
  }
}
