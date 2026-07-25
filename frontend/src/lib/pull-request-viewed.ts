import type { DiffFile } from "./diff"

// Which files of a pull request the reviewer has ticked off, so a long review
// keeps its place while the user jumps to the Overview tab and back. Keyed by
// the pull request (its URL, unique across projects) so two PRs never share
// ticks.
//
// A tick records *what* was read, not just that it was: each one stores a
// fingerprint of the file's diff, and a file only counts as viewed while its
// diff still hashes the same. A new commit therefore unticks exactly the files
// it touched — the way GitHub does it — instead of clearing the whole review or
// leaving stale ticks on changed files.
//
// In memory only: a restart clears every tick. Persisting them is now safe
// enough (the fingerprint catches a rewritten file), but a review that spans a
// restart has not been worth the storage.
//
// Module-level store + a subscribe/getSnapshot pair for useSyncExternalStore,
// matching the app's no-state-library pattern. Each snapshot is a map that is
// replaced, never mutated, so useSyncExternalStore sees a stable reference
// until something actually changes.
const byPullRequest = new Map<string, ReadonlyMap<string, string>>()
const listeners = new Set<() => void>()

const empty: ReadonlyMap<string, string> = new Map()

/** The ticks of one pull request, path → fingerprint; a stable reference between changes. */
export function viewedFiles(pullRequest: string): ReadonlyMap<string, string> {
  return byPullRequest.get(pullRequest) ?? empty
}

/** Tick a file off at its current fingerprint (or untick it). No-op — and no
 * notification — if it already reads that way. */
export function setViewed(
  pullRequest: string,
  path: string,
  fingerprint: string,
  viewed: boolean,
): void {
  const current = viewedFiles(pullRequest)
  if ((current.get(path) === fingerprint) === viewed) {
    return
  }
  const next = new Map(current)
  if (viewed) {
    next.set(path, fingerprint)
  } else {
    next.delete(path)
  }
  byPullRequest.set(pullRequest, next)
  for (const listener of listeners) {
    listener()
  }
}

export function subscribeViewed(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** Whether a file still reads as viewed — ticked off, and unchanged since. */
export function isViewed(
  viewed: ReadonlyMap<string, string>,
  path: string,
  fingerprint: string,
): boolean {
  return viewed.get(path) === fingerprint
}

// fileFingerprint identifies a file's diff content. Rename and status ride along
// so a file that moves, or flips added↔deleted, reads as something new to look
// at. Only the line text matters otherwise — hunk headers shift when an
// unrelated part of the file moves, and that is not a change worth unticking.
export function fileFingerprint(file: DiffFile): string {
  let hash = 0x811c9dc5
  const mix = (text: string): void => {
    for (let i = 0; i < text.length; i++) {
      hash ^= text.charCodeAt(i)
      // FNV-1a's 32-bit prime, via shifts because Math.imul on every byte of a
      // large diff is the same thing with more truncation.
      hash = (hash + (hash << 1) + (hash << 4) + (hash << 7) + (hash << 8) + (hash << 24)) >>> 0
    }
  }
  mix(file.status)
  mix(file.oldPath)
  mix(file.newPath)
  if (file.binary) {
    // No lines to read; the counts are all a binary file reports.
    mix(`bin:${file.added}:${file.deleted}`)
    return hash.toString(16)
  }
  for (const hunk of file.hunks) {
    for (const line of hunk.lines) {
      mix(line.kind)
      mix(line.text)
    }
  }
  return hash.toString(16)
}
