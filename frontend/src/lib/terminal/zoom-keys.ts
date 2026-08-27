// Zoom chords are matched on event.code — the physical key — not event.key.
//
// event.key carries the character the layout produces, and on every common
// layout (US, ABNT2, …) "+" is Shift+"=". A combo written as {shift: false,
// key: "+"} is therefore unsatisfiable: pressing what a user calls "Ctrl +"
// arrives as Ctrl+Shift+"=", the match fails, nothing calls preventDefault, and
// Chromium's own zoom accelerator runs instead. That is how the app ended up
// with two zooms at once — the app's for Ctrl+"−", Chromium's for Ctrl+"+".
//
// event.code is the same on every layout, so one table covers layouts where
// +/− live on Equal/Minus (US, ABNT2, …). Layouts with a dedicated "+" key
// (German: code BracketRight; its "-" sits on Slash) use the character fallback.
// These deliberately are not configurable: browser accelerators bind physical
// keys, not characters.

export type ZoomIntent = "in" | "out" | "reset"

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

// Shift is useful only on Equal, where it types "+". Elsewhere it would steal
// Ctrl+Shift+− or Ctrl+Shift+0 from the PTY for no gain.
const SHIFTABLE_CODE = "Equal"

export function zoomIntent(event: ZoomKeyState): ZoomIntent | null {
  if (!(event.ctrlKey || event.metaKey)) return null
  if (event.altKey) return null
  const byChar = ZOOM_CHARS[event.key]
  if (byChar) return byChar
  if (event.shiftKey && event.code !== SHIFTABLE_CODE) return null
  return ZOOM_CODES[event.code] ?? null
}
