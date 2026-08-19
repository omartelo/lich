import { useEffect, useRef, useSyncExternalStore } from "react"
import { createKeyedStore } from "@/lib/keyed-store"

// "Do this in the sidebar", raised by a global hotkey and honoured by the piece
// of sidebar chrome that owns the action — the worktree dialog, a card's rename
// field, its close and pin. Page-internal, like focus-request.
//
// A pending value rather than a tick: the chord expands a collapsed sidebar
// first, so its consumer can mount a render *after* the press, and a
// notification fired at press time would reach nobody. The value is cleared as
// it is read, so a request fires once and never springs the dialog open again
// the next time that consumer mounts.
//
// `empty` is both the idle value and what a consumed request resets to, so it
// is the one value a request may not carry.
function createIntent<T>(empty: T) {
  const store = createKeyedStore<T>(empty)
  return {
    request: (key: string, value: T) => store.set(key, value),
    // The action is written inline at the call site, so it is read through a
    // ref: depending on its identity would re-run the effect every render.
    useIntent(key: string, run: (value: T) => void): void {
      const latest = useRef(run)
      latest.current = run
      const pending = useSyncExternalStore(
        (onChange) => store.subscribe(key, onChange),
        () => store.get(key),
      )
      useEffect(() => {
        if (pending === empty) {
          return
        }
        store.set(key, empty)
        latest.current(pending)
      }, [pending, key])
    },
  }
}

// Keyed by project id: the dialog belongs to the project's sidebar.
const worktreeDialog = createIntent(false)
export const requestWorktreeDialog = (projectId: string) => worktreeDialog.request(projectId, true)
export const useWorktreeDialogIntent = worktreeDialog.useIntent

/** What a shortcut asks one session's card to do — the card's own actions. */
export type SessionIntent = "rename" | "close" | "pin" | "terminal" | "delegate"

// One store for the card actions rather than one per action: they share a key
// (the session id) and a consumer (the card), and a switch there reads better
// than five subscriptions.
const sessionAction = createIntent<SessionIntent | "">("")
export const requestSessionIntent = (sessionId: string, intent: SessionIntent) =>
  sessionAction.request(sessionId, intent)
export const useSessionIntent = sessionAction.useIntent
