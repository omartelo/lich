import { useKeyedStore } from "@/lib/use-keyed-store"
import { onAppEvent } from "@/lib/app-events"
import { RELAY_EVENT, STATUS_EVENT, type SessionRelay } from "./session-events"
import { createSessionRelayStore } from "./session-relay-store"

// Subscribed at import rather than on first use: that opens the /events socket
// at page load, so a request opened before any card mounts still lands.
const store = createSessionRelayStore(
  (handler) => onAppEvent(RELAY_EVENT, handler),
  (handler) => onAppEvent(STATUS_EVENT, handler),
)

// useSessionRelay reads the request a session has open with another from the
// shared store (see session-relay-store), which retains it across the card's
// unmount. Returns null while the session is on its own — the usual case, and
// what keeps the card its normal size.
export function useSessionRelay(id: string): SessionRelay | null {
  return useKeyedStore(store, id)
}
