import { onAppEvent } from "@/lib/app-events"
import { STATUS_EVENT, type SessionTool } from "./session-events"
import { createSessionToolStore } from "./session-tool-store"
import { useKeyedStore } from "@/lib/use-keyed-store"

// Subscribed at import rather than on first use: that opens the /events socket
// at page load, so a tool reported before any card mounts still lands.
const store = createSessionToolStore((handler) => onAppEvent(STATUS_EVENT, handler))

// useSessionTool reads the tool a session's turn is running from the shared
// store (see session-tool-store), which retains it across the card's unmount.
// Returns null whenever the session is not inside a tool call — which is most of
// the time, and is what leaves the card the size it was.
export function useSessionTool(sessionId: string): SessionTool | null {
  return useKeyedStore(store, sessionId)
}
