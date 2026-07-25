import { useCallback, useSyncExternalStore } from "react"
import { onAppEvent } from "./app-events"
import { USAGE_EVENT, type SessionUsage } from "./session-events"
import { createSessionUsageStore } from "./session-usage-store"

// Subscribed at import rather than on first use: that opens the /events socket
// at page load, so a usage report before any card mounts still lands.
const store = createSessionUsageStore((handler) => onAppEvent(USAGE_EVENT, handler))

// useSessionUsage reads a session's last reported context-window usage from the
// shared store (see session-usage-store), which retains it across the card's
// unmount. Returns null until the backend reports one — after the first turn
// ends, for a Claude session.
export function useSessionUsage(sessionId: string): SessionUsage | null {
  const subscribe = useCallback(
    (onChange: () => void) => store.subscribe(sessionId, onChange),
    [sessionId],
  )
  return useSyncExternalStore(subscribe, () => store.get(sessionId))
}
