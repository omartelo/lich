import { Store } from "@/lib/rpc"

// Whether the footer shows what a session has cost. The truth lives in the
// backend settings table, not in localStorage, because the flag gates work and
// not just paint: with it off no transcript is summed and no price is ever
// fetched (see internal/store settings.CostReadout). This store is the page's
// copy of it — loaded once, written through on every toggle — so turning the
// readout off clears the number immediately instead of at the next turn.
//
// Off is the honest default while the value is still loading: the figure only
// means something on API billing, so nobody should see one appear and vanish.

// SETTING_KEY and GLOBAL_SCOPE mirror the Go side's costReadoutKey and its
// global (project-less) scope.
const SETTING_KEY = "usage.cost"
const GLOBAL_SCOPE = ""

let enabled = false
let loaded = false
let ready = false
let revision = 0
let generation = 0
const readyListeners = new Set<() => void>()
const listeners = new Set<() => void>()

const notify = () => {
  for (const listener of listeners) {
    listener()
  }
}

// load reads the stored flag once per page. A failed read leaves it off, which
// is what an unreachable backend should show for a number about money.
const load = async () => {
  const startedAt = revision
  const loadGeneration = generation
  try {
    const value = await Store.GetSetting(SETTING_KEY, GLOBAL_SCOPE)
    if (loadGeneration !== generation) return
    const next = value === "true"
    if (startedAt === revision && next !== enabled) {
      enabled = next
      notify()
    }
  } catch {
    // Keep the default; the toggle in Settings still works and will write.
  } finally {
    if (loadGeneration === generation) {
      ready = true
      for (const listener of readyListeners) listener()
    }
  }
}

export const costReadoutStore = {
  get: (): boolean => enabled,
  isReady: (): boolean => ready,
  subscribeReady(listener: () => void): () => void {
    readyListeners.add(listener)
    const off = costReadoutStore.subscribe(() => {})
    return () => {
      readyListeners.delete(listener)
      off()
    }
  },
  subscribe(listener: () => void): () => void {
    listeners.add(listener)
    if (!loaded) {
      loaded = true
      void load()
    }
    return () => {
      listeners.delete(listener)
    }
  },
}

// setCostReadout flips the flag for good: the page updates at once and the
// backend stops (or starts) counting from its next turn.
export function setCostReadout(on: boolean): Promise<void> {
  const previous = enabled
  const writeRevision = ++revision
  if (on !== enabled) {
    enabled = on
    notify()
  }
  return Store.SetSetting(SETTING_KEY, GLOBAL_SCOPE, String(on)).then(
    () => {},
    (error) => {
      if (revision === writeRevision && enabled !== previous) {
        enabled = previous
        notify()
      }
      throw error
    },
  )
}

// resetCostReadoutStore drops the module state between tests.
export function resetCostReadoutStore(): void {
  enabled = false
  loaded = false
  ready = false
  revision++
  generation++
  readyListeners.clear()
  listeners.clear()
}
