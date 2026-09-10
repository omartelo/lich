// How many paths the conflict row names before it folds the rest away. Six fills
// one line of the header at an ordinary window width; past that the row stops
// answering faster and starts pushing the tabs down the screen.
export const NAMED = 6

// foldConflicts splits the list into what the row names inline and how many it
// is holding back — never a negative count, so the caller reads one number
// rather than a number and a comparison. Unfolded, the inline half is empty: the full list is drawn
// below it, and naming the first six twice would read as a repeat rather than as
// the same list opened.
export function foldConflicts(files: string[], all: boolean): { shown: string[]; hidden: number } {
  return { shown: all ? [] : files.slice(0, NAMED), hidden: Math.max(0, files.length - NAMED) }
}

// splitConflictPath cuts a path where the directory ends. A monorepo's
// conflicting files share most of their path, and the half that tells them apart
// is the last segment — so the two are drawn at different weights.
//
// A path with no directory is all name, and a path ending in a separator is all
// directory: neither is what GitHub sends, and both have to render as something.
export function splitConflictPath(file: string): { dir: string; name: string } {
  const cut = file.lastIndexOf("/") + 1
  return { dir: file.slice(0, cut), name: file.slice(cut) }
}
