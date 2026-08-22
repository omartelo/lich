import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import {
  isIdEvent,
  isIdleEvent,
  toSessionRelay,
  type SessionEventSource,
  type SessionRelay,
} from "./session-events"

// Two marks are the same when both fields match. The store rebuilds the object
// on every event, so without this a repeat report would hand
// useSyncExternalStore a new reference and re-render the card for nothing.
function sameRelay(a: SessionRelay | null, b: SessionRelay | null): boolean {
  if (a === null || b === null) {
    return a === b
  }
  return a.peer === b.peer && a.direction === b.direction && a.ticket === b.ticket
}

// createSessionRelayStore keeps the request each session has open with another,
// keyed by session id, fed by the relay event (see internal/relay). The backend
// raises a mark on both ends when a message lands in a PTY and takes both down
// when the errand closes, so this store only follows what it is told.
//
// It also clears on "idle": that is SessionEnd, the CLI leaving the PTY. A
// session that ended can no longer answer anything, and the mark would
// otherwise sit on a dead card until its next spawn — the relay's own clear
// only comes when someone answers or the ticket expires an hour later.
export function createSessionRelayStore(
  relaySource: SessionEventSource,
  statusSource: SessionEventSource,
): ReadableKeyedStore<SessionRelay | null> {
  const store = createKeyedStore<SessionRelay | null>(null, sameRelay)

  relaySource((data) => {
    if (isIdEvent(data)) {
      store.set(data.id, toSessionRelay(data))
    }
  })

  statusSource((data) => {
    if (isIdleEvent(data)) {
      store.set(data.id, null)
    }
  })

  return store
}
