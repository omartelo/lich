import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import { isCwdEvent, type SessionEventSource } from "./session-events"

// createSessionCwdStore keeps the last reported working directory of every
// session, keyed by session id, fed by one subscription taken at creation —
// before any card mounts. get() answers "" until the backend reports one, and
// the card falls back to the session's static path. The backend re-reports the
// start directory on every PTY spawn, so a respawn overwrites whatever the
// previous shell left here.
export function createSessionCwdStore(source: SessionEventSource): ReadableKeyedStore<string> {
  const store = createKeyedStore("")
  source((data) => {
    if (isCwdEvent(data)) {
      store.set(data.id, data.cwd)
    }
  })
  return store
}
