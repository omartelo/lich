import {
  isStatusEvent,
  toSessionStatus,
  type SessionStatus,
} from "./session-events"

// A subscription to the global status event, injected so the store is testable
// without standing up the /events socket. Returns its unsubscribe.
export type StatusEventSource = (
  handler: (data: unknown) => void,
) => () => void

// A session needing attention: blocked waiting on the user, or a turn that
// finished but has not been seen yet. The notification queue is a flat list of
// these across every project (see pendingAll).
export interface PendingStatus {
  id: string
  status: SessionStatus
}

function samePending(
  a: readonly PendingStatus[],
  b: readonly PendingStatus[],
): boolean {
  if (a.length !== b.length) {
    return false
  }
  return a.every((item, i) => item.id === b[i].id && item.status === b[i].status)
}

interface Entry {
  status: SessionStatus | null
  // Whether the user has had a chance to see the current status, which only
  // "done" cares about: it is the one state that persists with nothing running,
  // so a finished turn would badge its project tab forever. "busy" and
  // "waiting" are live — a tab left mid-run should keep saying so, and a
  // permission prompt left unanswered is still blocking.
  seen: boolean
  listeners: Set<() => void>
}

// The tab badge of a project with several sessions. "waiting" wins because it
// blocks the user, then "busy" because something is still running; "done" is
// the leftover.
const BADGE_PRIORITY = ["waiting", "busy", "done"] as const

// createSessionStatusStore keeps the last reported status of every session,
// keyed by session id, fed by one subscription taken at creation — before any
// card mounts.
//
// The status cannot live in the card: SessionSidebar only renders cards for the
// active project, so switching projects (or opening Home) unmounts every one of
// them. A useState there died with the card and took its event listener with it,
// so a status arriving while the card was unmounted was lost for good — coming
// back to the project showed no spinner for a session Claude was still working
// on. Entries therefore outlive their listeners: unsubscribing on unmount drops
// the listener, never the status.
export function createSessionStatusStore(source: StatusEventSource) {
  const entries = new Map<string, Entry>()

  const entryOf = (id: string): Entry => {
    let entry = entries.get(id)
    if (!entry) {
      entry = {status: null, seen: false, listeners: new Set()}
      entries.set(id, entry)
    }
    return entry
  }

  const notify = (entry: Entry): void => {
    for (const listener of entry.listeners) {
      listener()
    }
  }

  // The notification queue, recomputed on every status change and handed to
  // useSyncExternalStore. An unchanged set keeps the old array reference
  // (samePending), so subscribers never re-render for a transition that leaves
  // the queue identical — e.g. a session going busy, which the queue ignores.
  const globalListeners = new Set<() => void>()
  let pending: PendingStatus[] = []

  const computePending = (): PendingStatus[] => {
    const next: PendingStatus[] = []
    for (const [id, entry] of entries) {
      // "busy" is progress, not a notification; a seen "done" is already read.
      if (entry.status === null || entry.status === "busy") {
        continue
      }
      if (entry.status === "done" && entry.seen) {
        continue
      }
      next.push({id, status: entry.status})
    }
    return next
  }

  const refreshPending = (): void => {
    const next = computePending()
    if (samePending(pending, next)) {
      return
    }
    pending = next
    for (const listener of globalListeners) {
      listener()
    }
  }

  source((data) => {
    if (!isStatusEvent(data)) {
      return
    }
    const entry = entryOf(data.id)
    const next = toSessionStatus(data.state)
    // The snapshot is a string union, so identity is free: bail on a repeat
    // state and subscribers skip the re-render entirely.
    if (entry.status === next) {
      return
    }
    entry.status = next
    // A fresh report is by definition unseen, whether or not the last one was.
    entry.seen = false
    notify(entry)
    refreshPending()
  })

  // markSeen records that a session's status has been on screen — its project
  // was opened, or was open when the status arrived. Only a "done" changes
  // appearance from it (see Entry.seen), so only that notifies.
  const markSeen = (id: string): void => {
    const entry = entries.get(id)
    if (!entry || entry.seen) {
      return
    }
    entry.seen = true
    if (entry.status === "done") {
      notify(entry)
    }
    refreshPending()
  }

  // pendingOf reduces a project's sessions to the one status its tab should
  // badge, or null when there is nothing to say. A seen "done" says nothing.
  const pendingOf = (ids: readonly string[]): SessionStatus | null => {
    const live = new Set<SessionStatus>()
    for (const id of ids) {
      const entry = entries.get(id)
      if (!entry || entry.status === null) {
        continue
      }
      if (entry.status === "done" && entry.seen) {
        continue
      }
      live.add(entry.status)
    }
    return BADGE_PRIORITY.find((status) => live.has(status)) ?? null
  }

  const subscribe = (id: string, listener: () => void): (() => void) => {
    const entry = entryOf(id)
    entry.listeners.add(listener)
    return () => {
      entry.listeners.delete(listener)
    }
  }

  const get = (id: string): SessionStatus | null =>
    entries.get(id)?.status ?? null

  // subscribeAll fires whenever the notification queue changes (see pendingAll),
  // as opposed to subscribe, which is scoped to one session.
  const subscribeAll = (listener: () => void): (() => void) => {
    globalListeners.add(listener)
    return () => {
      globalListeners.delete(listener)
    }
  }

  // pendingAll returns the current queue. The reference is stable between
  // changes, so it is safe to hand straight to useSyncExternalStore.
  const pendingAll = (): PendingStatus[] => pending

  return {subscribe, get, markSeen, pendingOf, subscribeAll, pendingAll}
}
