// Feature flag for the xterm.js/WebGL proof of concept (see
// XtermTerminalView.tsx). Off by default; flip it from the inspector with
//   localStorage.setItem("lich.xtermPoc", "1")
// and reload. The POC exists to benchmark xterm+WebGL against the patched
// ghostty-web pipeline before deciding a migration — remove the flag and the
// component once the decision lands.

export const XTERM_POC_KEY = "lich.xtermPoc"

/** True when the xterm.js POC flag is set. Storage injectable for tests. */
export function isXtermPocEnabled(storage?: Pick<Storage, "getItem"> | null): boolean {
  return storage?.getItem(XTERM_POC_KEY) === "1"
}
