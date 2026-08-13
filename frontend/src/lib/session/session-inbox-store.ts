import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import { isIdEvent, toInboxCount } from "./session-events"

// A subscription to one of the global session events, injected so the store is
// testable without standing up the /events socket. Returns its unsubscribe.
export type InboxEventSource = (handler: (data: unknown) => void) => () => void

// createSessionInboxStore keeps how many uncollected results each session has
// waiting in the relay's inbox, keyed by session id (see relay.InboxEventName).
// The nudge tells the agent; this is what tells the person, on the card.
//
// It clears on "idle" — SessionEnd — the way the relay mark does: a session
// that ended collects nothing, and the count would otherwise sit on a dead
// card until the entries expire an hour later.
export function createSessionInboxStore(
  inboxSource: InboxEventSource,
  statusSource: InboxEventSource,
): ReadableKeyedStore<number> {
  const store = createKeyedStore<number>(0)

  inboxSource((data) => {
    if (isIdEvent(data)) {
      store.set(data.id, toInboxCount(data))
    }
  })

  statusSource((data) => {
    if (isIdEvent(data) && (data as { state?: unknown }).state === "idle") {
      store.set(data.id, 0)
    }
  })

  return store
}
