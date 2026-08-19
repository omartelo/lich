// Files dropped on a terminal, turned into paths at its prompt.
//
// A native terminal emulator pastes the path of what was dropped on it; the
// shell here is a Chromium window, and Chromium hands the page the bytes but
// never the path (no `text/uri-list`, no `File.path` — measured, not assumed).
// So the backend takes it from here: it looks the item up by name, size and
// mtime — under the session's directory, then under home (internal/drop) — and
// only what it cannot find is uploaded and pasted as a copy.
//
// A confined session (internal/sandbox) takes the copy far more often, and has
// to: its home is an empty private one, so the backend does not look there for
// it at all, and everything outside its checkout arrives as a copy — which is
// the one thing lich writes where such a session can still read it.
//
// The provider on the other end does the rest — a pasted image path is
// attached as an image, any other path is read as a path.

import { DropService, endpoint } from "@/lib/rpc"
import type { DropItem } from "@/lib/api-types"
import { bracketedPaste } from "./bracketed-paste"

// Matches internal/drop's maxUpload: bigger than a screenshot or a log is a
// mis-drag, and the backend refuses it anyway.
const MAX_UPLOAD_BYTES = 32 << 20

// Matches internal/drop's keepDropped: how long a copy outlives its drop, which
// is the half of the notice the user cannot find out any other way.
export const KEEP_DROPPED_DAYS = 3

/** One dropped entry: what the page can say about it, plus its bytes. */
export interface DroppedFile extends DropItem {
  /** Null for a directory — the page has no bytes to upload for one. */
  blob: Blob | null
}

/**
 * The drop's entries, read while the event is still live. DataTransfer is
 * neutered the moment the handler awaits anything, so every field — including
 * the directory flag, which only `webkitGetAsEntry` carries — has to be taken
 * out synchronously. The Blob references survive.
 */
export function readDroppedFiles(transfer: DataTransfer): DroppedFile[] {
  const dropped: DroppedFile[] = []
  for (const item of transfer.items) {
    if (item.kind !== "file") {
      continue
    }
    const file = item.getAsFile()
    if (!file) {
      continue
    }
    const dir = item.webkitGetAsEntry?.()?.isDirectory ?? false
    dropped.push({
      name: file.name,
      size: dir ? 0 : file.size,
      mtime: file.lastModified,
      dir,
      blob: dir ? null : file,
    })
  }
  return dropped
}

/**
 * The session a drop landed on: the tree the backend searches, the id the
 * copies are kept under (they are deleted with the session), and whether it
 * runs in the sandbox.
 */
export interface DropTarget {
  cwd: string
  sessionId: string
  confined: boolean
}

export interface DropResult {
  /** Absolute paths, in the order they were dropped. */
  paths: string[]
  /** Names of entries that yielded no path, with why. */
  skipped: string[]
  /**
   * Names of entries neither tree held, whose path is a copy's — an edit there
   * never reaches the user's file, and the copy expires. Nothing else in the UI
   * distinguishes those paths from the real ones.
   */
  copied: string[]
}

/**
 * Paths for a drop: the real one where the session's tree holds the file, a
 * copy's otherwise.
 */
export async function resolveDroppedFiles(
  target: DropTarget,
  dropped: readonly DroppedFile[],
): Promise<DropResult> {
  const paths: string[] = []
  const skipped: string[] = []
  const copied: string[] = []
  if (dropped.length === 0) {
    return { paths, skipped, copied }
  }
  const items = dropped.map(({ name, size, mtime, dir }) => ({ name, size, mtime, dir }))
  const found = await DropService.Resolve(target.cwd, items, target.confined)
  for (const [index, entry] of dropped.entries()) {
    const path = found[index] ?? ""
    if (path !== "") {
      paths.push(path)
      continue
    }
    // Nothing to fall back to: a directory cannot be uploaded, and copying one
    // to paste the copy's path is not the drop the user made.
    if (entry.dir || !entry.blob) {
      skipped.push(`${entry.name} (${missingFolder(target.confined)})`)
      continue
    }
    if (entry.blob.size > MAX_UPLOAD_BYTES) {
      skipped.push(`${entry.name} (over ${MAX_UPLOAD_BYTES >> 20}MB)`)
      continue
    }
    try {
      paths.push(await uploadDroppedFile(target.sessionId, entry.name, entry.blob))
      copied.push(entry.name)
    } catch {
      skipped.push(entry.name)
    }
  }
  return { paths, skipped, copied }
}

// missingFolder is why a dropped folder yielded no path, which is a different
// sentence for a confined session: its home was never searched, so "not found
// under your home" would name a search that did not happen.
function missingFolder(confined: boolean): string {
  return confined
    ? "folder outside this sandboxed session's checkout"
    : "folder not found under this session or your home"
}

// uploadDroppedFile stores one file's bytes on the backend and answers with the
// path of the copy. Its own endpoint, not the RPC: the body is the file. The
// session id rides along because the copy is kept under it and deleted with it.
async function uploadDroppedFile(sessionId: string, name: string, blob: Blob): Promise<string> {
  const { base, token } = endpoint()
  const query = `token=${token}&session=${encodeURIComponent(sessionId)}`
  const url = `${base}/drop?${query}&name=${encodeURIComponent(name)}`
  const response = await fetch(url, { method: "POST", body: blob })
  if (!response.ok) {
    throw new Error(`drop upload: HTTP ${response.status}`)
  }
  const body = (await response.json()) as { path?: string }
  if (!body.path) {
    throw new Error("drop upload: no path in response")
  }
  return body.path
}

/**
 * The paths as one terminal write, quoted and space-separated the way a
 * terminal emulator pastes a drop — with a trailing space, so what the user
 * types next does not run into the last path. Empty for an empty drop, which
 * the caller must not write.
 */
export function composeDroppedPaths(paths: readonly string[], isWindows = false): string {
  if (paths.length === 0) {
    return ""
  }
  return bracketedPaste(`${paths.map((path) => quotePath(path, isWindows)).join(" ")} `)
}

// quotePath keeps a path with spaces — or anything else a shell would act on —
// a single argument, because the prompt underneath may well be a shell. The
// safe set covers both separators, so an ordinary path of either OS is written
// bare. Beyond it the quote is the host's: cmd.exe knows only double quotes
// (and a Windows path cannot contain one), POSIX gets single quotes, where a
// path holding one closes, escapes it and reopens.
function quotePath(path: string, isWindows: boolean): string {
  if (/^[\w@%+=:,./\\-]+$/.test(path)) {
    return path
  }
  if (isWindows) {
    return `"${path}"`
  }
  return `'${path.split("'").join(`'\\''`)}'`
}
