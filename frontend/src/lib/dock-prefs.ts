import { parseEnumPref, readPref, writePref } from "@/lib/prefs"

// What the right dock remembers between its mounts. The dock is a ternary
// between two component types and sits behind a close button, so every
// Code↔Review switch and every close destroys the panel — a choice made once
// was a choice re-made all day.
//
// UI preferences, so the page's localStorage rather than the workspace database
// (see the root CLAUDE.md). Parsing is the half kept pure and tested; reading
// and writing the key stays a two-line wrapper, the split prefs.ts describes.
//
// Global rather than per session, by the rule pulls-prefs states: which changes
// a reviewer wants in front of them is a habit of theirs, the way the pull
// request sort is, and not a property of one checkout's content.
const SOURCE_KEY = "lich.dock.source"

/** Which changes the Review tab shows. "worktree" is everything uncommitted,
 * the panel's original and default answer; "turn" narrows it to the window the
 * session's last finished turn ran in (internal/terminal.LastTurnDiff). */
export const DIFF_SOURCES = ["worktree", "turn"] as const
export type DiffSource = (typeof DIFF_SOURCES)[number]

export function readDiffSource(): DiffSource {
  return parseEnumPref(readPref(SOURCE_KEY), DIFF_SOURCES, "worktree")
}

export function writeDiffSource(source: DiffSource): void {
  writePref(SOURCE_KEY, source)
}
