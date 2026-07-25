// Zoom chords are matched on event.code — the physical key — not event.key.
//
// event.key carries the *character* the layout produces, and on every common
// layout (US, ABNT2, …) "+" is Shift+"=". A combo written as {shift: false,
// key: "+"} is therefore unsatisfiable: pressing what a user calls "Ctrl +"
// arrives as Ctrl+Shift+"=", the match fails, nothing calls preventDefault, and
// Chromium's own zoom accelerator runs instead. That is how the app ended up
// with two zooms at once — the app's for Ctrl+"−", Chromium's for Ctrl+"+".
//
// event.code is the same on every layout, so one table covers the layouts
// where +/− live on the Equal/Minus keys (US, ABNT2, …). Layouts with a
// dedicated "+" key (German: code BracketRight; its "-" sits on Slash) miss
// the table, so the character is checked as a fallback — there event.key
// already says what the press means. These deliberately are not
// user-configurable hotkeys: the whole point is to shadow the browser
// accelerators, and an accelerator is bound to a physical key, not to a
// character.

export type ZoomIntent = "in" | "out" | "reset"

// The subset of KeyboardEvent the matcher needs — lets tests pass plain
// objects, same shape trick as hotkeys.ts and term-keys.ts.
export type ZoomKeyState = Pick<
  KeyboardEvent,
  "ctrlKey" | "metaKey" | "shiftKey" | "altKey" | "code" | "key"
>

const ZOOM_CODES: Record<string, ZoomIntent> = {
  Equal: "in",
  NumpadAdd: "in",
  Minus: "out",
  NumpadSubtract: "out",
  Digit0: "reset",
  Numpad0: "reset",
}

const ZOOM_CHARS: Record<string, ZoomIntent> = {
  "+": "in",
  "-": "out",
}

// Shift is only accepted on Equal, because there it is what types the "+".
// Everywhere else it would just steal another chord (Ctrl+Shift+−, Ctrl+Shift+0)
// from the PTY for no gain.
const SHIFTABLE_CODE = "Equal"

// zoomIntent reports which zoom a keypress asks for, or null when it is not a
// zoom chord. The caller is expected to preventDefault so Chromium does not
// zoom on top of the app.
export function zoomIntent(event: ZoomKeyState): ZoomIntent | null {
  if (!(event.ctrlKey || event.metaKey)) return null
  if (event.altKey) return null
  const byChar = ZOOM_CHARS[event.key]
  if (byChar) return byChar
  if (event.shiftKey && event.code !== SHIFTABLE_CODE) return null
  return ZOOM_CODES[event.code] ?? null
}
