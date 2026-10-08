import { onAppEvent } from "@/lib/app-events"
import { useKeyedStore } from "@/lib/use-keyed-store"
import { LIMIT_EVENT, STATUS_EVENT, type SessionLimit } from "./session-events"
import { createSessionLimitStore } from "./session-limit-store"

// Subscribed at import, like the relay store: a limit reported while the card
// is unmounted still has to be there when it mounts.
const store = createSessionLimitStore(
  (handler) => onAppEvent(LIMIT_EVENT, handler),
  (handler) => onAppEvent(STATUS_EVENT, handler),
)

// useSessionLimit reads the usage limit a session's last turn ended on, or null
// while none holds.
export function useSessionLimit(id: string): SessionLimit | null {
  return useKeyedStore(store, id)
}
