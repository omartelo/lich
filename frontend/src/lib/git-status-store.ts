export interface GitStatus {
  branch: string
  files: number
  added: number
  deleted: number
}

export type GitStatusFetcher = (path: string) => Promise<GitStatus | null>

interface Entry {
  status: GitStatus | null
  listeners: Set<() => void>
  timer: ReturnType<typeof setInterval>
  // Fetches this path once, off the interval cadence. Reused for the
  // visibilitychange re-fetch and the store's manual refresh(path).
  refresh: () => void
}

const unchanged = (a: GitStatus | null, b: GitStatus | null): boolean =>
  a === b ||
  (a !== null &&
    b !== null &&
    a.branch === b.branch &&
    a.files === b.files &&
    a.added === b.added &&
    a.deleted === b.deleted)

// createGitStatusStore shares one poll loop per path across every subscriber.
// Before this, each session card ran its own interval: 20 cards on the same
// project meant ~90 git subprocesses and 20+ React re-renders bursting every
// 3s even with a clean, idle repo — the 35-50ms idle rAF stalls. One poller
// per path plus keeping the same object identity when nothing changed makes
// the idle case one fetch per path and zero re-renders.
export function createGitStatusStore(fetch: GitStatusFetcher, pollMs: number) {
  const entries = new Map<string, Entry>()

  const subscribe = (path: string, listener: () => void): (() => void) => {
    let entry = entries.get(path)
    if (!entry) {
      const refresh = () => {
        if (typeof document !== "undefined" && document.hidden) {
          return
        }
        void fetch(path).then((next) => {
          // Drop the result if every subscriber left mid-flight or the status
          // is identical: same object identity means subscribers skip their
          // re-render entirely.
          if (entries.get(path) !== created || unchanged(created.status, next)) {
            return
          }
          created.status = next
          for (const notify of created.listeners) {
            notify()
          }
        })
      }
      const created: Entry = {
        status: null,
        listeners: new Set(),
        timer: setInterval(refresh, pollMs),
        refresh,
      }
      entries.set(path, created)
      if (typeof document !== "undefined") {
        document.addEventListener("visibilitychange", created.refresh)
      }
      refresh()
      entry = created
    }
    entry.listeners.add(listener)
    return () => {
      entry.listeners.delete(listener)
      if (entry.listeners.size > 0) {
        return
      }
      clearInterval(entry.timer)
      if (typeof document !== "undefined") {
        document.removeEventListener("visibilitychange", entry.refresh)
      }
      entries.delete(path)
    }
  }

  const get = (path: string): GitStatus | null =>
    entries.get(path)?.status ?? null

  // refresh fetches a path now, ahead of its poll tick. A no-op when nothing is
  // subscribed to that path (no card is showing it), so an event for a session
  // in a background project costs no git call.
  const refresh = (path: string): void => {
    entries.get(path)?.refresh()
  }

  return {subscribe, get, refresh}
}
