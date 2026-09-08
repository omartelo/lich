import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import { isCwdEvent, type SessionEventSource } from "./session-events"

// Where a session is, as far as the backend can tell. At most one side is set:
// a host means the shell runs somewhere no local reader follows (tmux, ssh, a
// container), so there is no directory to report rather than a stale one.
export interface SessionCwd {
  cwd: string
  host: string
}

const NOWHERE: SessionCwd = { cwd: "", host: "" }

// createSessionCwdStore keeps the last reported location of every session,
// keyed by session id, fed by one subscription taken at creation — before any
// card mounts. get() answers an empty pair until the backend reports one, and
// the card falls back to the session's static path. The backend re-reports the
// start directory on every PTY spawn, so a respawn overwrites whatever the
// previous shell left here.
//
// The value is an object rebuilt on every report, so the store is given its own
// comparison: without it every poll would re-render the card (see keyed-store).
export function createSessionCwdStore(source: SessionEventSource): ReadableKeyedStore<SessionCwd> {
  const store = createKeyedStore<SessionCwd>(
    NOWHERE,
    (a, b) => a.cwd === b.cwd && a.host === b.host,
  )
  source((data) => {
    if (isCwdEvent(data)) {
      store.set(data.id, { cwd: data.cwd, host: data.host ?? "" })
    }
  })
  return store
}
