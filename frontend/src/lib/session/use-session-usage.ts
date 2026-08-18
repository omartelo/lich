import { onAppEvent } from "@/lib/app-events"
import { AGENT_EVENT, STATUS_EVENT, USAGE_EVENT, type SessionUsage } from "./session-events"
import { createSessionUsageStore } from "./session-usage-store"
import { useKeyedStore } from "@/lib/use-keyed-store"

// Subscribed at import rather than on first use: that opens the /events socket
// at page load, so a usage report before any card mounts still lands.
const store = createSessionUsageStore(
  (handler) => onAppEvent(USAGE_EVENT, handler),
  (handler) => onAppEvent(AGENT_EVENT, handler),
  (handler) => onAppEvent(STATUS_EVENT, handler),
)

// useSessionUsage reads a session's last reported context-window usage from the
// shared store (see session-usage-store), which retains it across the card's
// unmount but clears it when that provider exits or the PTY respawns. Returns
// null until the backend reports one.
export function useSessionUsage(sessionId: string): SessionUsage | null {
  return useKeyedStore(store, sessionId)
}
