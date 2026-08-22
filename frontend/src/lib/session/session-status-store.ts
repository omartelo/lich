import {
  isStatusEvent,
  statusReason,
  toSessionStatus,
  type SessionEventSource,
  type SessionStatus,
} from "./session-events"

// A session needing attention: blocked waiting on the user, or a turn that
// finished but has not been seen yet. The notification queue is a flat list of
// these across every project (see pendingAll).
export interface PendingStatus {
  id: string
  status: SessionStatus
}

function samePending(a: readonly PendingStatus[], b: readonly PendingStatus[]): boolean {
  if (a.length !== b.length) {
    return false
  }
  return a.every((item, i) => item.id === b[i].id && item.status === b[i].status)
}

interface Entry {
  status: SessionStatus | null
  // What the session is blocked on, in the provider's own words, and "" for
  // every state that is not a question. It rides here rather than in a store of
  // its own because "waiting" is the only thing that clears it: the reason and
  // the state it belongs to leave together, and holding them apart would be two
  // entries that must never disagree.
  reason: string
  // When the current status started, as wall-clock ms — what the card's elapsed
  // readout counts from. Stamped on the transition alone: the hook reports the
  // same state repeatedly while a turn runs, and a session has been waiting
  // since it started waiting, not since the last report said so again.
  since: number
  // Whether the user has read the current status, which only "done" cares
  // about: it is the one state that persists with nothing running, so a
  // finished turn would badge its project tab forever and its ring would say
  // "back from the agent" long after it was read. "busy" and "waiting" are
  // live — a tab left mid-run should keep saying so, and a permission prompt
  // left unanswered is still blocking. Reading is per session and not per
  // project: the card whose terminal is on screen is the one that was looked
  // at, and the sessions beside it in the sidebar are exactly the ones whose
  // results nobody has collected yet (see the provider's markSessionSeen).
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
export function createSessionStatusStore(source: SessionEventSource) {
  const entries = new Map<string, Entry>()

  const entryOf = (id: string): Entry => {
    let entry = entries.get(id)
    if (!entry) {
      entry = { status: null, reason: "", since: 0, seen: false, listeners: new Set() }
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
      next.push({ id, status: entry.status })
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
    const reason = next === "waiting" ? statusReason(data) : ""
    // The snapshot is a string union, so identity is free: bail on a repeat
    // state and subscribers skip the re-render entirely. The reason is weighed
    // with it because a second permission prompt inside one turn repeats the
    // state and changes the question — the one repeat that is news.
    if (entry.status === next && entry.reason === reason) {
      return
    }
    // Stamped on the state's own transition alone: a session has been waiting
    // since it started waiting, not since the question it is waiting on changed.
    if (entry.status !== next) {
      entry.since = Date.now()
    }
    entry.status = next
    entry.reason = reason
    // A fresh report is by definition unseen, whether or not the last one was.
    entry.seen = false
    notify(entry)
    refreshPending()
  })

  // markSeen records that a session's status has been read — its card was
  // focused, or was on screen and being watched when the status arrived. Only a
  // "done" changes appearance from it (see Entry.seen), so only that notifies.
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
  // badge, or null when there is nothing to say. A read "done" says nothing.
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

  // runningOf returns the sessions of these ids that still hold a turn: an agent
  // working, or one blocked on a prompt nobody has answered. Unlike pendingOf
  // this ignores "seen" — a turn does not stop running because it was looked at
  // — and it answers with the sessions themselves, since a caller asking this
  // is about to say how many are at stake.
  const runningOf = (ids: readonly string[]): string[] =>
    ids.filter((id) => {
      const status = entries.get(id)?.status
      return status === "busy" || status === "waiting"
    })

  const subscribe = (id: string, listener: () => void): (() => void) => {
    const entry = entryOf(id)
    entry.listeners.add(listener)
    return () => {
      entry.listeners.delete(listener)
    }
  }

  const get = (id: string): SessionStatus | null => entries.get(id)?.status ?? null

  // unread is the one question the card's ring asks beyond the status itself:
  // a turn that finished and has not been looked at since. Only "done" can be
  // unread — the live states say what they say whether or not anyone is
  // watching — so everything else answers false.
  const unread = (id: string): boolean => {
    const entry = entries.get(id)
    return entry?.status === "done" && !entry.seen
  }

  // reason answers what a waiting session is blocked on, "" when nothing was
  // reported — a provider whose event carries no words for it, or any state that
  // is not a question (docs/hooks/session-state.md).
  const reason = (id: string): string => entries.get(id)?.reason ?? ""

  // since answers when the current status started, or null when there is no
  // status to time — including for an entry a subscriber opened before the
  // first report landed.
  const since = (id: string): number | null => {
    const entry = entries.get(id)
    return entry && entry.status !== null ? entry.since : null
  }

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

  return {
    subscribe,
    get,
    unread,
    reason,
    since,
    markSeen,
    pendingOf,
    runningOf,
    subscribeAll,
    pendingAll,
  }
}
