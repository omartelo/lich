import type { BaseStatus } from "@/lib/api-types"

export interface GitStatus {
  branch: string
  files: number
  added: number
  deleted: number
  /** The HEAD commit — how a subscriber notices a commit, not just an edit. */
  head: string
  /** How the checkout stands against the branch it merges into; null when the
   * repository has no origin to measure against. */
  base: BaseStatus | null
}

export type GitStatusFetcher = (path: string) => Promise<GitStatus | null>

// How often a path is re-read. A repository being worked in is polled at
// fastMs; after idleTicks reads that changed nothing it drops to slowMs, and
// the first change puts it back on fastMs. One read is several git
// subprocesses, so a flat fast interval would burn them all day on checkouts
// nobody is touching — the cost belongs where the work is.
export interface PollCadence {
  fastMs: number
  slowMs: number
  idleTicks: number
}

interface Entry {
  status: GitStatus | null
  listeners: Set<() => void>
  timer: ReturnType<typeof setTimeout> | undefined
  // Fetches this path once, off the poll cadence. Reused for the
  // visibilitychange re-fetch and the store's manual refresh(path).
  refresh: () => void
  // Monotonic fetch id — only the latest-started fetch may publish.
  seq: number
  // Consecutive reads that found nothing new; at idleTicks the poll slows down.
  quiet: number
}

// The conflict list is compared by content, not by length: two merges can
// collide on the same number of files and not the same files, and a badge that
// kept the old paths would name files the checkout no longer fights over.
const sameBase = (a: BaseStatus | null, b: BaseStatus | null): boolean =>
  a === b ||
  (a != null &&
    b != null &&
    a.base === b.base &&
    a.behind === b.behind &&
    (a.conflicts ?? []).join("\n") === (b.conflicts ?? []).join("\n"))

const unchanged = (a: GitStatus | null, b: GitStatus | null): boolean =>
  a === b ||
  (a !== null &&
    b !== null &&
    a.branch === b.branch &&
    a.files === b.files &&
    a.added === b.added &&
    a.deleted === b.deleted &&
    a.head === b.head &&
    sameBase(a.base, b.base))

// createGitStatusStore shares one poll loop per path across every subscriber.
// Before this, each session card ran its own interval: 20 cards on the same
// project meant ~90 git subprocesses and 20+ React re-renders bursting every
// 3s even with a clean, idle repo — the 35-50ms idle rAF stalls. One poller
// per path plus keeping the same object identity when nothing changed makes
// the idle case one fetch per path and zero re-renders.
export function createGitStatusStore(fetch: GitStatusFetcher, cadence: PollCadence) {
  const entries = new Map<string, Entry>()

  const subscribe = (path: string, listener: () => void): (() => void) => {
    let entry = entries.get(path)
    if (!entry) {
      // A hidden window polls nothing: the next read comes from the
      // visibilitychange listener, which starts the loop up again.
      const refresh = () => {
        if (typeof document !== "undefined" && document.hidden) {
          return
        }
        const seq = ++created.seq
        void fetch(path)
          .then((next) => {
            // Drop the result if every subscriber left mid-flight or a newer
            // fetch started since.
            if (entries.get(path) !== created || seq !== created.seq) {
              return
            }
            // An identical status keeps its object identity, so subscribers
            // skip their re-render entirely — and the path counts as quiet.
            if (unchanged(created.status, next)) {
              created.quiet++
              return
            }
            created.quiet = 0
            created.status = next
            for (const notify of created.listeners) {
              notify()
            }
          })
          .catch(() => {
            // A fetcher that rejects must not end the loop.
          })
          .finally(() => {
            if (entries.get(path) === created) {
              schedule()
            }
          })
      }
      // Scheduled off the freshest read rather than on a fixed interval: the
      // cadence follows how busy the repository is, and a read never overlaps
      // the one before it. Clearing first keeps an out-of-band refresh (the
      // session-touched nudge) from leaving a second timer behind.
      const schedule = () => {
        clearTimeout(created.timer)
        created.timer = setTimeout(
          refresh,
          created.quiet >= cadence.idleTicks ? cadence.slowMs : cadence.fastMs,
        )
      }
      const created: Entry = {
        status: null,
        listeners: new Set(),
        timer: undefined,
        refresh,
        seq: 0,
        quiet: 0,
      }
      entries.set(path, created)
      if (typeof document !== "undefined") {
        document.addEventListener("visibilitychange", created.refresh)
      }
      // The first read schedules the next one when it lands.
      refresh()
      entry = created
    }
    entry.listeners.add(listener)
    return () => {
      entry.listeners.delete(listener)
      if (entry.listeners.size > 0) {
        return
      }
      clearTimeout(entry.timer)
      if (typeof document !== "undefined") {
        document.removeEventListener("visibilitychange", entry.refresh)
      }
      entries.delete(path)
    }
  }

  const get = (path: string): GitStatus | null => entries.get(path)?.status ?? null

  // refresh fetches a path now, ahead of its poll tick. A no-op when nothing is
  // subscribed to that path (no card is showing it), so an event for a session
  // in a background project costs no git call.
  const refresh = (path: string): void => {
    entries.get(path)?.refresh()
  }

  return { subscribe, get, refresh }
}
