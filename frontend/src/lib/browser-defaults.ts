// The shell is a browser only by construction, and its reflexes leak into an
// --app window that has no tabs, no address bar and no way back: the accelerators
// still fire, a dropped file still navigates. One of each is fatal — Ctrl+W
// closes the window, which quits lich, and a file drop replaces the app with the
// file, with no address bar to return from.
//
// They only reach the browser when nothing on the page claims them first, which
// is why they misfire everywhere except a focused terminal — xterm already
// preventDefaults the chords it encodes.

// The subset of KeyboardEvent the matcher needs — lets tests pass plain objects.
type ChordState = Pick<KeyboardEvent, "ctrlKey" | "metaKey" | "shiftKey" | "altKey" | "key">

// Tab and window commands (new/close/restore), the file and page commands whose
// dialogs or navigation have nowhere to land, and the browsing-data wipe, which
// would take lich's own localStorage settings with it.
const MOD_KEYS = new Set(["t", "w", "n", "p", "s", "o", "u", "d", "h", "j"])
const MOD_SHIFT_KEYS = new Set(["t", "w", "n", "m", "delete"])

// Deliberately still live: Ctrl+R and F5 (a reload recovers a wedged UI and
// costs nothing — the session lives in the backend), the devtools chords
// (Ctrl+Shift+I/J/C, F12) and F11. Alt is excluded throughout because AltGr
// arrives as Ctrl+Alt on Windows and Linux layouts, where it types characters.
export function isBrowserChord(event: ChordState): boolean {
  const mod = event.ctrlKey || event.metaKey
  if (!mod || event.altKey) {
    return false
  }
  const key = event.key.toLowerCase()
  return event.shiftKey ? MOD_SHIFT_KEYS.has(key) : MOD_KEYS.has(key)
}

// isAppContextMenu reports a right-click on the app's own chrome — everything
// outside a terminal. The terminal keeps Chromium's menu because that is where
// its Copy and Paste entries live; the rest of the UI has no such use for it and
// would only be offered Back, Reload, Save as, Print and View source.
export function isAppContextMenu(target: EventTarget | null): boolean {
  const element = target as HTMLElement | null
  return !element?.closest?.(".xterm")
}

// installBrowserDefaults swallows the lot. preventDefault only, never
// stopPropagation: a terminal must still receive every chord as a PTY sequence,
// and most of these are real terminal bindings (Ctrl+W kills a word, Ctrl+U a
// line, Ctrl+D is EOF, Ctrl+P walks shell history).
export function installBrowserDefaults(target: Window): void {
  target.addEventListener(
    "keydown",
    (event) => {
      if (isBrowserChord(event)) {
        event.preventDefault()
      }
    },
    true,
  )
  target.addEventListener("contextmenu", (event) => {
    if (isAppContextMenu(event.target)) {
      event.preventDefault()
    }
  })
  // Bubble phase, so a future drop zone of our own runs first and this only
  // catches what nothing claimed. dnd-kit is untouched — it rides pointer
  // events, not the HTML drag protocol.
  target.addEventListener("dragover", (event) => event.preventDefault())
  target.addEventListener("drop", (event) => event.preventDefault())
}
