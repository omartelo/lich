import {isCwdEvent} from "./session-events"

// A subscription to the global cwd event, injected so the store is testable
// without standing up the /events socket. Returns its unsubscribe.
export type CwdEventSource = (handler: (data: unknown) => void) => () => void

interface Entry {
  cwd: string
  listeners: Set<() => void>
}

// createSessionCwdStore keeps the last reported working directory of every
// session, keyed by session id, fed by one subscription taken at creation —
// before any card mounts. Same shape as session-status-store, for the same
// reason: cards unmount with their project, so the value cannot live in them.
// The backend re-reports the start directory on every PTY spawn, so a respawn
// overwrites whatever the previous shell left here.
export function createSessionCwdStore(source: CwdEventSource) {
  const entries = new Map<string, Entry>()

  const entryOf = (id: string): Entry => {
    let entry = entries.get(id)
    if (!entry) {
      entry = {cwd: "", listeners: new Set()}
      entries.set(id, entry)
    }
    return entry
  }

  source((data) => {
    if (!isCwdEvent(data)) {
      return
    }
    const entry = entryOf(data.id)
    // Snapshots are plain strings, so identity is free: bail on a repeat and
    // subscribers skip the re-render entirely.
    if (entry.cwd === data.cwd) {
      return
    }
    entry.cwd = data.cwd
    for (const listener of entry.listeners) {
      listener()
    }
  })

  const subscribe = (id: string, listener: () => void): (() => void) => {
    const entry = entryOf(id)
    entry.listeners.add(listener)
    return () => {
      entry.listeners.delete(listener)
    }
  }

  // get returns "" while nothing has been reported — the card then falls back
  // to the session's static path.
  const get = (id: string): string => entries.get(id)?.cwd ?? ""

  return {subscribe, get}
}
