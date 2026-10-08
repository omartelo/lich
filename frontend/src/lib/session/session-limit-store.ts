import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import {
  isIdEvent,
  isStatusEvent,
  type SessionEventSource,
  type SessionLimit,
  toSessionLimit,
} from "./session-events"

function sameLimit(a: SessionLimit | null, b: SessionLimit | null): boolean {
  if (a === null || b === null) {
    return a === b
  }
  return a.window === b.window && a.resetsAt === b.resetsAt
}

// createSessionLimitStore keeps the usage limit each session's last turn ended
// on, keyed by session id. A turn starting ("busy") is the proof the limit no
// longer holds, and the CLI leaving ("idle") takes the session with it; nothing
// else clears it, because the turn's own end can reach the window on either
// side of the limit (internal/terminal, limit.go).
export function createSessionLimitStore(
  limitSource: SessionEventSource,
  statusSource: SessionEventSource,
): ReadableKeyedStore<SessionLimit | null> {
  const store = createKeyedStore<SessionLimit | null>(null, sameLimit)

  limitSource((data) => {
    if (isIdEvent(data)) {
      store.set(data.id, toSessionLimit(data))
    }
  })

  statusSource((data) => {
    if (isStatusEvent(data) && (data.state === "busy" || data.state === "idle")) {
      store.set(data.id, null)
    }
  })

  return store
}
