import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import { isIdEvent, isIdleEvent, toInboxCount, type SessionEventSource } from "./session-events"

// createSessionInboxStore keeps how many uncollected results each session has
// waiting in the relay's inbox, keyed by session id (see relay.InboxEventName).
// The nudge tells the agent; this is what tells the person, on the card.
//
// It clears on "idle" — SessionEnd — the way the relay mark does: a session
// that ended collects nothing, and the count would otherwise sit on a dead
// card until the entries expire an hour later.
export function createSessionInboxStore(
  inboxSource: SessionEventSource,
  statusSource: SessionEventSource,
): ReadableKeyedStore<number> {
  const store = createKeyedStore<number>(0)

  inboxSource((data) => {
    if (isIdEvent(data)) {
      store.set(data.id, toInboxCount(data))
    }
  })

  statusSource((data) => {
    if (isIdleEvent(data)) {
      store.set(data.id, 0)
    }
  })

  return store
}
