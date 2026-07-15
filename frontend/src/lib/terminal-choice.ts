// Terminal emulator choice. xterm.js + WebGL is the terminal since phase 3 of
// docs/chromium-shell.md; the patched ghostty-web pipeline stays available as
// an escape hatch until phase 5 deletes it:
//   localStorage.setItem("lich.terminal", "ghostty")  // + reload

export const TERMINAL_CHOICE_KEY = "lich.terminal"

/** True when the user pinned the legacy ghostty-web terminal. */
export function useGhosttyTerminal(storage?: Pick<Storage, "getItem"> | null): boolean {
  return storage?.getItem(TERMINAL_CHOICE_KEY) === "ghostty"
}
