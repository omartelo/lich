import { createKeyedStore, type ReadableKeyedStore } from "./keyed-store"
import { isAgentEvent, isStatusEvent } from "./session-events"
import { isSessionKind, type SessionKind } from "./sessions"

// A subscription to one of the global session events, injected so the store is
// testable without standing up the /events socket. Returns its unsubscribe.
export type AgentEventSource = (handler: (data: unknown) => void) => () => void

// createSessionAgentStore keeps the provider CLI currently live inside each
// session's PTY, keyed by session id — the hand-run `claude` in a shell
// session that its card should wear the mark of. Fed by two subscriptions
// taken at creation, before any card mounts:
//
// - the agent event sets the mark (an empty or unknown agent clears it — the
//   backend emits "" on every PTY spawn so a respawn drops a stale mark);
// - a status report of "idle" clears it: that is SessionEnd, Claude leaving
//   the PTY, and the card falls back to its own kind.
//
// Never persisted: the mark is live PTY state, like the cwd.
export function createSessionAgentStore(
  agentSource: AgentEventSource,
  statusSource: AgentEventSource,
): ReadableKeyedStore<SessionKind | null> {
  const store = createKeyedStore<SessionKind | null>(null)

  agentSource((data) => {
    if (!isAgentEvent(data)) {
      return
    }
    // The payload crosses a process boundary: anything but a known kind — the
    // clearing "", a provider from a newer backend — falls back to no mark.
    store.set(data.id, isSessionKind(data.agent) ? data.agent : null)
  })

  statusSource((data) => {
    if (isStatusEvent(data) && data.state === "idle") {
      store.set(data.id, null)
    }
  })

  return store
}
