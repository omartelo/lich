import { useKeyedStore } from "@/lib/use-keyed-store"
import { onAppEvent } from "@/lib/app-events"
import { INBOX_EVENT, STATUS_EVENT } from "./session-events"
import { createSessionInboxStore } from "./session-inbox-store"

// Subscribed at import rather than on first use: that opens the /events socket
// at page load, so a result stashed before any card mounts still lands.
const store = createSessionInboxStore(
  (handler) => onAppEvent(INBOX_EVENT, handler),
  (handler) => onAppEvent(STATUS_EVENT, handler),
)

// useSessionInbox reads how many uncollected results a session has waiting.
// Zero — the usual case — draws nothing.
export function useSessionInbox(id: string): number {
  return useKeyedStore(store, id)
}
