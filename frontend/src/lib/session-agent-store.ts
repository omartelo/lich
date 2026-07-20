import {isAgentEvent, isStatusEvent} from "./session-events"
import {isSessionKind, type SessionKind} from "./sessions"

// A subscription to one of the global session events, injected so the store is
// testable without standing up the /events socket. Returns its unsubscribe.
export type AgentEventSource = (handler: (data: unknown) => void) => () => void

interface Entry {
  agent: SessionKind | null
  listeners: Set<() => void>
}

// createSessionAgentStore keeps the provider CLI currently live inside each
// session's PTY, keyed by session id — the hand-run `claude` in a shell
// session that its card should wear the mark of. Fed by two subscriptions
// taken at creation, before any card mounts (same shape as
// session-status-store, for the same unmount reason):
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
) {
  const entries = new Map<string, Entry>()

  const entryOf = (id: string): Entry => {
    let entry = entries.get(id)
    if (!entry) {
      entry = {agent: null, listeners: new Set()}
      entries.set(id, entry)
    }
    return entry
  }

  const set = (id: string, agent: SessionKind | null): void => {
    const entry = entryOf(id)
    if (entry.agent === agent) {
      return
    }
    entry.agent = agent
    for (const listener of entry.listeners) {
      listener()
    }
  }

  agentSource((data) => {
    if (!isAgentEvent(data)) {
      return
    }
    // The payload crosses a process boundary: anything but a known kind — the
    // clearing "", a provider from a newer backend — falls back to no mark.
    set(data.id, isSessionKind(data.agent) ? data.agent : null)
  })

  statusSource((data) => {
    if (!isStatusEvent(data) || data.state !== "idle") {
      return
    }
    set(data.id, null)
  })

  const subscribe = (id: string, listener: () => void): (() => void) => {
    const entry = entryOf(id)
    entry.listeners.add(listener)
    return () => {
      entry.listeners.delete(listener)
    }
  }

  const get = (id: string): SessionKind | null => entries.get(id)?.agent ?? null

  return {subscribe, get}
}
