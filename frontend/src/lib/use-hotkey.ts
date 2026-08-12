import { useEffect, useRef } from "react"
import { isRecordingTarget, matchesCombo, type Combo } from "@/lib/hotkeys"

// One place for what every global shortcut has to get right: the listener sits
// in the window capture phase so the chord is seen before the terminal that has
// focus, and the event is stopped there so the PTY never receives it — a hotkey
// that reached the shell would run a command as well as its action. Recording a
// rebind in Settings bails out, so pressing a combo to store it does not also
// fire the action it is being stored for.
//
// The handler may decline by returning false, for the shortcuts whose action is
// unavailable right now (no project open): the chord then behaves as if lich had
// never bound it and falls through to whatever is underneath.
export function useHotkey(combo: Combo, handler: () => unknown): void {
  // The handler is read through a ref so a caller may pass an inline closure
  // without re-subscribing the listener on every render.
  const latest = useRef(handler)
  latest.current = handler
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (isRecordingTarget(event) || !matchesCombo(event, combo)) {
        return
      }
      if (latest.current() === false) {
        return
      }
      event.preventDefault()
      event.stopPropagation()
    }
    window.addEventListener("keydown", onKey, true)
    return () => window.removeEventListener("keydown", onKey, true)
  }, [combo])
}
