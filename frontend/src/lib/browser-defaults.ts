// Chromium keeps browser accelerators in --app mode even though there is no
// browser chrome to recover with. This guard owns only the browser default: it
// never stops propagation, so a chord with no lich binding still reaches xterm.

type ChordState = Pick<KeyboardEvent, "ctrlKey" | "metaKey" | "shiftKey" | "altKey" | "key">

const MOD_KEYS: Record<string, true> = {
  t: true,
  w: true,
  n: true,
  p: true,
  s: true,
  o: true,
  u: true,
  d: true,
  h: true,
  j: true,
  q: true,
  l: true,
  f: true,
  "[": true,
  "]": true,
  "+": true,
  "=": true,
  "-": true,
  "0": true,
}

const MOD_SHIFT_KEYS: Record<string, true> = {
  t: true,
  w: true,
  n: true,
  m: true,
  p: true,
  o: true,
  q: true,
  delete: true,
  "+": true,
  "=": true,
}

export function isBrowserChord(event: ChordState): boolean {
  const key = event.key.toLowerCase()
  const mod = event.ctrlKey || event.metaKey
  if (mod && !event.altKey) {
    return event.shiftKey ? MOD_SHIFT_KEYS[key] === true : MOD_KEYS[key] === true
  }
  if (!mod && event.altKey && !event.shiftKey) {
    return key === "arrowleft" || key === "arrowright" || key === "f4"
  }
  return false
}

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
  // A future drop zone runs first; this only catches what nothing claimed.
  target.addEventListener("dragover", (event) => event.preventDefault())
  target.addEventListener("drop", (event) => event.preventDefault())
}
