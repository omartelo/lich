import { isMac } from "@/lib/platform"

// Chromium reflexes leak into an --app window with no tabs, address bar or way
// back. Ctrl+W closes the window and quits lich; a dropped file replaces the
// app. The latter can also strand the localStorage-backed settings.
//
// The guard owns browser defaults only. It prevents them in capture phase but
// never stops propagation, so xterm still receives every chord.
type ChordState = Pick<KeyboardEvent, "ctrlKey" | "metaKey" | "shiftKey" | "altKey" | "key">

// Tab/window commands, file/page surfaces with nowhere useful to land, the
// browsing-data wipe, Find and Chromium zoom. Shift changes the browser action;
// it does not make the chord harmless.
const MOD_KEYS = new Set([
  "t",
  "w",
  "n",
  "p",
  "s",
  "o",
  "u",
  "d",
  "h",
  "j",
  "q",
  "l",
  "f",
  "+",
  "=",
  "-",
  "0",
])
const MOD_SHIFT_KEYS = new Set(["t", "w", "n", "m", "p", "o", "q", "delete", "+", "="])

// Ctrl+R/F5, DevTools, F11 and Alt+F4 remain live. A renderer cannot reliably
// take Alt+F4 back from the window manager. Alt is excluded from Ctrl/Meta
// guards because AltGr arrives as Ctrl+Alt on Windows and Linux.
export function isBrowserChord(event: ChordState, mac = isMac): boolean {
  const key = event.key.toLowerCase()
  const mod = event.ctrlKey || event.metaKey
  if (mod && !event.altKey) {
    if (mac && event.metaKey && !event.ctrlKey && !event.shiftKey && (key === "[" || key === "]")) {
      return true
    }
    return event.shiftKey ? MOD_SHIFT_KEYS.has(key) : MOD_KEYS.has(key)
  }
  if (!mac && !mod && event.altKey && !event.shiftKey) {
    return key === "arrowleft" || key === "arrowright"
  }
  return false
}

// Chromium's terminal menu supplies Copy and Paste. Elsewhere it offers only
// browser actions such as Back, Reload, Save as, Print and View source.
export function isAppContextMenu(target: EventTarget | null): boolean {
  const element = target as HTMLElement | null
  return !element?.closest?.(".xterm")
}

export function installBrowserDefaults(target: Window): void {
  target.addEventListener(
    "keydown",
    (event) => {
      if (isBrowserChord(event)) event.preventDefault()
    },
    true,
  )
  target.addEventListener("contextmenu", (event) => {
    if (isAppContextMenu(event.target)) event.preventDefault()
  })
  // Bubble phase lets a future drop zone claim the event first. dnd-kit uses
  // pointer events, not the HTML drag protocol.
  target.addEventListener("dragover", (event) => event.preventDefault())
  target.addEventListener("drop", (event) => event.preventDefault())
}
