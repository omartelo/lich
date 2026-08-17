import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import {
  isStatusEvent,
  statusTool,
  type SessionEventSource,
  type SessionTool,
} from "./session-events"

// Two reports name the same tool when both fields match. The store rebuilds the
// object on every event, so without this a repeat report would hand
// useSyncExternalStore a new reference and re-render the card for nothing.
function sameTool(a: SessionTool | null, b: SessionTool | null): boolean {
  if (a === null || b === null) {
    return a === b
  }
  return a.name === b.name && a.detail === b.detail
}

// createSessionToolStore keeps the tool each session's turn is running, keyed by
// session id, fed by the status event the same hook already reports on (see
// docs/hooks/session-state.md). It reads that event rather than the status store
// for two reasons: a repeat "busy" is exactly what carries a *new* tool, and the
// status store collapses repeats into no change at all.
//
// The clearing rule is the whole of this store:
//
// - a state other than "busy" ends the tool, whatever the payload says;
// - a "busy" naming a tool sets it;
// - a "busy" naming none changes nothing. PostToolUse reports one between every
//   two tools, so treating it as a clear would blink the line off and on at
//   every step of a turn.
//
// Leaving "busy" is what clears, never "idle" alone: Codex has no SessionEnd
// event and would otherwise hold a dead tool name until its next PTY spawn.
export function createSessionToolStore(
  statusSource: SessionEventSource,
): ReadableKeyedStore<SessionTool | null> {
  const store = createKeyedStore<SessionTool | null>(null, sameTool)

  statusSource((data) => {
    if (!isStatusEvent(data)) {
      return
    }
    if (data.state !== "busy") {
      store.set(data.id, null)
      return
    }
    const tool = statusTool(data)
    if (tool) {
      store.set(data.id, tool)
    }
  })

  return store
}
