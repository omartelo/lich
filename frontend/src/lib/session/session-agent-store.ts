import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import { isAgentEvent, isIdleEvent, type SessionEventSource } from "./session-events"
import { isSessionKind, type SessionKind } from "./sessions"

// createSessionAgentStore keeps the provider CLI currently live inside each
// session's PTY, keyed by session id — the hand-run `claude` or `codex` in a
// shell session that its card should wear the mark of. Fed by two subscriptions
// taken at creation, before any card mounts:
//
// - the agent event sets the mark (an empty or unknown agent clears it — the
//   backend emits "" on every PTY spawn so a respawn drops a stale mark);
// - a status report of "idle" clears it: that is SessionEnd, the CLI leaving
//   the PTY, and the card falls back to its own kind. A provider with no such
//   event (Codex) keeps its mark until the next spawn.
//
// Never persisted: the mark is live PTY state, like the cwd.
export function createSessionAgentStore(
  agentSource: SessionEventSource,
  statusSource: SessionEventSource,
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
    if (isIdleEvent(data)) {
      store.set(data.id, null)
    }
  })

  return store
}
