import {useCallback, useSyncExternalStore} from "react"
import {onAppEvent} from "./app-events"
import {AGENT_EVENT, STATUS_EVENT} from "./session-events"
import {createSessionAgentStore} from "./session-agent-store"
import type {SessionKind} from "./sessions"

// Subscribed at import rather than on first use: that opens the /events socket
// at page load, so an agent reported before any card mounts still lands.
const store = createSessionAgentStore(
  (handler) => onAppEvent(AGENT_EVENT, handler),
  (handler) => onAppEvent(STATUS_EVENT, handler),
)

// useSessionAgent reads the provider CLI currently live inside a session's PTY
// from the shared store (see session-agent-store), which retains it across the
// card's unmount. Returns null while nothing runs there — the card then shows
// its own kind.
export function useSessionAgent(sessionId: string): SessionKind | null {
  const subscribe = useCallback(
    (onChange: () => void) => store.subscribe(sessionId, onChange),
    [sessionId],
  )
  return useSyncExternalStore(subscribe, () => store.get(sessionId))
}
