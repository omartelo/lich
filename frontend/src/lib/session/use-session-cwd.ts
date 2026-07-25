import { onAppEvent } from "@/lib/app-events"
import { CWD_EVENT } from "./session-events"
import { createSessionCwdStore } from "./session-cwd-store"
import { useKeyedStore } from "@/lib/use-keyed-store"

// Subscribed at import rather than on first use: that opens the /events socket
// at page load, so a cwd reported before any card mounts still lands.
const store = createSessionCwdStore((handler) => onAppEvent(CWD_EVENT, handler))

// useSessionCwd reads a session's live working directory from the shared store
// (see session-cwd-store), which retains it across the card's unmount. Returns
// "" while the backend has reported nothing for the session.
export function useSessionCwd(sessionId: string): string {
  return useKeyedStore(store, sessionId)
}
