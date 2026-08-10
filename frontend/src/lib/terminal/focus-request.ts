// "Put the cursor back in this session's terminal", raised by chrome outside the
// terminal — the sidebar writing a mention at the prompt — and honoured by the
// TerminalView that owns the session. Page-internal: app-events carries what the
// backend has to say, never what one part of the page asks of another.
//
// The value is a tick rather than a state because focus is an event: two
// requests in a row have to notify twice, and a store that skips an equal value
// would swallow the second.

import { createKeyedStore } from "@/lib/keyed-store"

const ticks = createKeyedStore<number>(0)

export function requestTerminalFocus(sessionId: string): void {
  ticks.set(sessionId, ticks.get(sessionId) + 1)
}

export function onTerminalFocusRequest(sessionId: string, listener: () => void): () => void {
  return ticks.subscribe(sessionId, listener)
}
