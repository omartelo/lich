import { useCallback, useEffect, useState, useSyncExternalStore } from "react"
import { onAppEvent } from "@/lib/app-events"
import { STATUS_EVENT, type SessionStatus } from "./session-events"
import { createSessionStatusStore, type PendingStatus } from "./session-status-store"
import { formatAge, subscribeAge } from "./session-age"
import { useKeyedStore } from "@/lib/use-keyed-store"

// Subscribed at import rather than on first use: that opens the /events socket
// at page load, so a status reported before any card mounts still lands.
const store = createSessionStatusStore((handler) => onAppEvent(STATUS_EVENT, handler))

// useSessionStatus reads a session's last reported Claude Code processing state
// from the shared store (see session-status-store), which retains it across the
// card's unmount. Returns null when nothing has been reported for the session,
// and whenever the report maps to no indicator (see toSessionStatus).
export function useSessionStatus(sessionId: string): SessionStatus | null {
  return useKeyedStore(store, sessionId)
}

// useSessionStatusAge returns how long the session has been in its current
// status, short and relative ("40s", "12m", "3h"), or "" for a status with no
// clock worth reading.
//
// "busy" and "waiting" have one: both are live, and with several agents running
// how long each has been in that state is what picks the one to deal with
// first. "done" does not — the turn is over, nothing is accruing, and its
// number would climb for hours on a card that has nothing left to say, pulling
// the eye away from the cards that do.
export function useSessionStatusAge(sessionId: string): string {
  const status = useSessionStatus(sessionId)
  const subscribe = useCallback(
    (onChange: () => void) => store.subscribe(sessionId, onChange),
    [sessionId],
  )
  const since = useSyncExternalStore(subscribe, () => store.since(sessionId))
  const started = status === "busy" || status === "waiting" ? since : null
  const [now, setNow] = useState(() => Date.now())
  // One timer for every card on screen (see subscribeAge); a card in any other
  // state subscribes to nothing at all.
  useEffect(() => {
    if (started === null) {
      return
    }
    return subscribeAge(started, setNow)
  }, [started])
  return started === null ? "" : formatAge(now - started)
}

// markSessionSeen records that a session's status has been on screen, so a
// finished turn stops badging its project's tab. Called from the provider,
// outside React's render.
export function markSessionSeen(sessionId: string): void {
  store.markSeen(sessionId)
}

// useProjectStatus reduces a project's sessions to the single status its tab
// badges — what is happening in there while you are looking somewhere else.
// The snapshot is a string union, so useSyncExternalStore is safe without
// memoizing it: equal statuses are identical values.
export function useProjectStatus(sessionIds: readonly string[]): SessionStatus | null {
  // sessionIds is a fresh array on every render of the tab strip; its contents
  // are what actually matter to the subscription.
  const key = sessionIds.join(",")
  const subscribe = useCallback(
    (onChange: () => void) => {
      const offs = sessionIds.map((id) => store.subscribe(id, onChange))
      return () => {
        for (const off of offs) {
          off()
        }
      }
    },
    [key],
  )
  const snapshot = useCallback(() => store.pendingOf(sessionIds), [key])
  return useSyncExternalStore(subscribe, snapshot)
}

// usePendingStatuses returns every session needing attention across all
// projects — the notification queue (see session-status-store.pendingAll).
// subscribeAll and pendingAll are module-stable, so no memoization is needed.
export function usePendingStatuses(): PendingStatus[] {
  return useSyncExternalStore(store.subscribeAll, store.pendingAll)
}
