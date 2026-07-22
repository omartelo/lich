import {useCallback, useSyncExternalStore} from "react"
import {onAppEvent} from "./app-events"
import {STATUS_EVENT, type SessionStatus} from "./session-events"
import {createSessionStatusStore, type PendingStatus} from "./session-status-store"

// Subscribed at import rather than on first use: that opens the /events socket
// at page load, so a status reported before any card mounts still lands.
const store = createSessionStatusStore((handler) =>
  onAppEvent(STATUS_EVENT, handler),
)

// useSessionStatus reads a session's last reported Claude Code processing state
// from the shared store (see session-status-store), which retains it across the
// card's unmount. Returns null when nothing has been reported for the session,
// and whenever the report maps to no indicator (see toSessionStatus).
export function useSessionStatus(sessionId: string): SessionStatus | null {
  const subscribe = useCallback(
    (onChange: () => void) => store.subscribe(sessionId, onChange),
    [sessionId],
  )
  return useSyncExternalStore(subscribe, () => store.get(sessionId))
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
export function useProjectStatus(
  sessionIds: readonly string[],
): SessionStatus | null {
  // sessionIds is a fresh array on every render of the tab strip; its contents
  // are what actually matter to the subscription.
  const key = sessionIds.join(",")
  const subscribe = useCallback(
    (onChange: () => void) => {
      const offs = sessionIds.map((id) => store.subscribe(id, onChange))
      return () => offs.forEach((off) => off())
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [key],
  )
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const snapshot = useCallback(() => store.pendingOf(sessionIds), [key])
  return useSyncExternalStore(subscribe, snapshot)
}

// usePendingStatuses returns every session needing attention across all
// projects — the notification queue (see session-status-store.pendingAll).
// subscribeAll and pendingAll are module-stable, so no memoization is needed.
export function usePendingStatuses(): PendingStatus[] {
  return useSyncExternalStore(store.subscribeAll, store.pendingAll)
}
