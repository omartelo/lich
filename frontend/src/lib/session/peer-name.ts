// The name a session answers to in Claude Code's peer roster — the list its
// cross-session messaging addresses, and what `/list-agents` prints. lich passes
// it at spawn (terminal.Start) and shows the same string on the card, so both
// come from here: derive it twice and the tooltip would name a session by an
// address that reaches a different one.
//
// Left to itself Claude names a session after its working directory, which is
// the same directory for every session lich runs in one checkout — the id tail
// is what tells those apart.

// How much of the session id trails the directory name. Four is enough to
// separate the handful of sessions one checkout holds, and short enough to stay
// readable in a roster row.
const ID_CHARS = 4

export function peerName(cwd: string, id: string): string {
  // Windows separators too: the name is built the same way wherever lich runs,
  // even though only macOS and Linux have the messaging feature.
  const last = cwd
    .replace(/[\\/]+$/, "")
    .split(/[\\/]/)
    .pop()
  // "." names no directory, and the Go half reads a path that names nothing the
  // same way: filepath.Base answers "." for it, and falls back to the app name.
  const dir = !last || last === "." ? "lich" : last
  const tail = id.replace(/[^a-z0-9]/gi, "").slice(0, ID_CHARS)
  return tail ? `${dir}-${tail}` : dir
}

/**
 * How a target is named at the sender's prompt. The trailing space is what the
 * drop of a file does too: whatever the user types next must not run into it.
 */
export function peerMention(name: string): string {
  return `@${name} `
}
