// Pure unified-diff parsing for the review panel: no DOM, no CodeMirror, so
// everything here runs under vitest's node environment.

export type DiffLineKind = "add" | "del" | "context" | "meta"

export interface DiffLine {
  kind: DiffLineKind
  text: string
  /** Line number in the old file; null for added and meta lines. */
  oldLine: number | null
  /** Line number in the new file; null for deleted and meta lines. */
  newLine: number | null
}

export interface DiffHunk {
  header: string
  oldStart: number
  oldCount: number
  newStart: number
  newCount: number
  lines: DiffLine[]
}

export type DiffFileStatus = "modified" | "added" | "deleted" | "renamed"

export interface DiffFile {
  oldPath: string
  newPath: string
  status: DiffFileStatus
  binary: boolean
  added: number
  deleted: number
  hunks: DiffHunk[]
}

const HUNK_HEADER = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/
const DIFF_GIT = /^diff --git a\/(.*) b\/(.*)$/

function newFile(oldPath: string, newPath: string): DiffFile {
  return {
    oldPath,
    newPath,
    status: "modified",
    binary: false,
    added: 0,
    deleted: 0,
    hunks: [],
  }
}

// stripPathPrefix turns "a/src/foo.ts" or "b/src/foo.ts" into "src/foo.ts",
// leaving "/dev/null" untouched so status detection can key off it.
function stripPathPrefix(path: string): string {
  return path.startsWith("a/") || path.startsWith("b/") ? path.slice(2) : path
}

function parseHunkHeader(line: string): DiffHunk | null {
  const match = HUNK_HEADER.exec(line)
  if (!match) {
    return null
  }
  return {
    header: line,
    oldStart: Number(match[1]),
    oldCount: match[2] === undefined ? 1 : Number(match[2]),
    newStart: Number(match[3]),
    newCount: match[4] === undefined ? 1 : Number(match[4]),
    lines: [],
  }
}

// applyFileHeader mutates the file being assembled according to one header
// line, returning true when the line was a header (and thus consumed).
function applyFileHeader(file: DiffFile, line: string): boolean {
  if (line.startsWith("new file mode")) {
    file.status = "added"
  } else if (line.startsWith("deleted file mode")) {
    file.status = "deleted"
  } else if (line.startsWith("rename from ")) {
    file.status = "renamed"
    file.oldPath = line.slice("rename from ".length)
  } else if (line.startsWith("rename to ")) {
    file.newPath = line.slice("rename to ".length)
  } else if (line.startsWith("Binary files ") || line.startsWith("GIT binary patch")) {
    file.binary = true
  } else if (line.startsWith("--- ")) {
    const path = stripPathPrefix(line.slice(4))
    if (path === "/dev/null") {
      file.status = "added"
    } else if (file.status !== "renamed") {
      file.oldPath = path
    }
  } else if (line.startsWith("+++ ")) {
    const path = stripPathPrefix(line.slice(4))
    if (path === "/dev/null") {
      file.status = "deleted"
    } else if (file.status !== "renamed") {
      file.newPath = path
    }
  } else if (
    !line.startsWith("index ") &&
    !line.startsWith("similarity index ") &&
    !line.startsWith("dissimilarity index ") &&
    !line.startsWith("old mode ") &&
    !line.startsWith("new mode ")
  ) {
    return false
  }
  return true
}

// appendHunkLine classifies one line inside a hunk and advances the running
// old/new line counters.
function appendHunkLine(
  file: DiffFile,
  hunk: DiffHunk,
  line: string,
  counters: { old: number; new: number },
): void {
  if (line.startsWith("+")) {
    hunk.lines.push({ kind: "add", text: line, oldLine: null, newLine: counters.new })
    counters.new += 1
    file.added += 1
  } else if (line.startsWith("-")) {
    hunk.lines.push({ kind: "del", text: line, oldLine: counters.old, newLine: null })
    counters.old += 1
    file.deleted += 1
  } else if (line.startsWith("\\")) {
    hunk.lines.push({ kind: "meta", text: line, oldLine: null, newLine: null })
  } else {
    hunk.lines.push({ kind: "context", text: line, oldLine: counters.old, newLine: counters.new })
    counters.old += 1
    counters.new += 1
  }
}

// parseDiff walks git's unified diff output line by line and splits it into
// per-file structures with old/new line numbers resolved for every hunk line.
export function parseDiff(text: string): DiffFile[] {
  const files: DiffFile[] = []
  let file: DiffFile | null = null
  let hunk: DiffHunk | null = null
  const counters = { old: 0, new: 0 }

  for (const line of text.split("\n")) {
    const started = DIFF_GIT.exec(line)
    if (started) {
      file = newFile(started[1], started[2])
      files.push(file)
      hunk = null
      continue
    }
    if (!file) {
      continue
    }
    const header = parseHunkHeader(line)
    if (header) {
      hunk = header
      file.hunks.push(hunk)
      counters.old = hunk.oldStart
      counters.new = hunk.newStart
      continue
    }
    if (hunk) {
      appendHunkLine(file, hunk, line, counters)
      continue
    }
    applyFileHeader(file, line)
  }
  return files
}

// FileDoc is the CodeMirror document for one file: the diff's code lines with
// their +/-/space prefixes stripped, hunks separated by one blank spacer line,
// plus per-line metadata (lineMeta[i] describes doc line i+1) so selections
// map back to file lines without re-parsing. In lineMeta, kind "meta" means
// exclusively "hunk separator" — "\ No newline" markers are omitted entirely.
export interface FileDoc {
  text: string
  lineMeta: DiffLine[]
  /** The unchanged runs git never printed, in document order — what the panel
   * offers to pull in. Empty when every gap is either closed or unnumberable. */
  gaps: DiffGap[]
}

/** One run of unchanged lines missing from the diff: the space between two
 * hunks, or the head of the file above the first one. The tail is not among
 * them — a diff carries no file length, so nothing here knows whether there is
 * anything past the last hunk. */
export interface DiffGap {
  /** The new-file line this gap opened at, before anything was pulled in. It
   * is the gap's identity: the doc is rebuilt around every expansion, and this
   * is what still names the same gap afterwards. */
  key: number
  /** 1-based document line the separator sits on — where the affordance goes. */
  docLine: number
  /** New-file lines still missing, inclusive. */
  from: number
  to: number
  /** The old-file line pairing with `from`. Unchanged text advances both sides
   * in step, and a line pulled in has to be numbered on both. */
  oldFrom: number
}

/** What has been pulled into each gap so far, keyed by DiffGap.key and always
 * counting from the gap's own start. */
export type Expansions = ReadonlyMap<number, DiffLine[]>

const hunkSeparator: DiffLine = { kind: "meta", text: "", oldLine: null, newLine: null }

// gapBefore measures the unchanged run between two hunk headers — or above the
// first one, with `previous` null. It answers null when there is nothing to
// pull in, and also when the two sides disagree about the run's length: a gap
// is unchanged text, so old and new must advance by the same amount, and a
// header pair saying otherwise is one this cannot number.
function gapBefore(previous: DiffHunk | null, hunk: DiffHunk): DiffGap | null {
  const from = previous ? previous.newStart + previous.newCount : 1
  const oldFrom = previous ? previous.oldStart + previous.oldCount : 1
  const to = hunk.newStart - 1
  if (to < from || hunk.oldStart - oldFrom !== hunk.newStart - from) {
    return null
  }
  return { key: from, docLine: 0, from, to, oldFrom }
}

// contextLines numbers text pulled out of the file into the diff's own line
// shape: unchanged lines, present on both sides, starting at newFrom/oldFrom.
export function contextLines(texts: string[], newFrom: number, oldFrom: number): DiffLine[] {
  return texts.map((text, index) => ({
    kind: "context" as const,
    text,
    oldLine: oldFrom + index,
    newLine: newFrom + index,
  }))
}

// appendExpansion folds one answer into what a gap has pulled in so far.
//
// Appended, never replaced: a gap wider than the backend's cap comes in several
// answers, each starting where the last one stopped. Which is also why an answer
// that does *not* start there is dropped whole — two expand clicks on the same
// gap can be in flight at once (a PR's head is read over the network), and both
// resolve against the gap as it was before either landed. Appending the second
// would write the same lines twice, gutter numbers and all, with nothing on a
// PR or turn diff to refetch them away.
export function appendExpansion(
  held: Expansions,
  gap: DiffGap,
  texts: readonly string[],
): Expansions {
  const pulled = held.get(gap.key) ?? []
  if (gap.from !== gap.key + pulled.length) {
    return held
  }
  const next = new Map(held)
  next.set(gap.key, [...pulled, ...contextLines([...texts], gap.from, gap.oldFrom)])
  return next
}

// buildFileDoc assembles the document, laying whatever has been pulled into a
// gap where the diff left a hole. A gap that is still open keeps its separator
// line and is reported in `gaps`; one that has been filled in whole has no
// separator left, because the text either side of it is now continuous.
export function buildFileDoc(file: DiffFile, expansions?: Expansions): FileDoc {
  const lines: string[] = []
  const lineMeta: DiffLine[] = []
  const gaps: DiffGap[] = []
  const push = (line: DiffLine): void => {
    lines.push(line.text)
    lineMeta.push(line)
  }
  for (const [index, hunk] of file.hunks.entries()) {
    const gap = gapBefore(index === 0 ? null : file.hunks[index - 1], hunk)
    const pulled = gap ? (expansions?.get(gap.key) ?? []) : []
    for (const line of pulled) {
      push(line)
    }
    const open = gap && {
      ...gap,
      from: gap.from + pulled.length,
      oldFrom: gap.oldFrom + pulled.length,
    }
    // Adjacent hunks (no numberable gap) keep the separator they always had.
    if (open ? open.from <= open.to : index > 0) {
      push(hunkSeparator)
      if (open) {
        gaps.push({ ...open, docLine: lineMeta.length })
      }
    }
    for (const line of hunk.lines) {
      if (line.kind === "meta") {
        continue
      }
      // lineMeta's text must equal the doc's, so decoration position math
      // (pos += text.length + 1) stays in step.
      push({ ...line, text: line.text.slice(1) })
    }
  }
  return { text: lines.join("\n"), lineMeta, gaps }
}

// discardTargets lists the repo-relative paths a "discard changes" on this
// file must revert: just the file itself, except a rename needs both sides —
// removing the new path and restoring the old one.
export function discardTargets(file: DiffFile): string[] {
  return file.status === "renamed" && file.oldPath !== file.newPath
    ? [file.newPath, file.oldPath]
    : [file.newPath]
}

// gutterNumber renders the single-column line gutter: deleted lines show
// their old-file number, everything else the new-file one, separators nothing.
// Either number is nullable by type, and a missing one renders as nothing rather
// than as the string "null" — parseDiff always numbers the side it keeps, but
// this also draws hunks assembled elsewhere (a GitHub diffHunk, thread-hunk.ts).
export function gutterNumber(line: DiffLine): string {
  const number = line.kind === "del" ? line.oldLine : line.newLine
  return number === null ? "" : String(number)
}

export interface NewLineRange {
  start: number
  end: number
}

// newLineRange maps a doc-line selection to the covered range of NEW-file
// lines: the min/max of every non-null newLine in the span (adds + context),
// naturally spanning hunk gaps. A selection holding only deleted/meta lines
// has no new-file range and yields null.
export function newLineRange(
  lineMeta: DiffLine[],
  fromDocLine: number,
  toDocLine: number,
): NewLineRange | null {
  let start = Infinity
  let end = -Infinity
  for (const meta of lineMeta.slice(fromDocLine - 1, toDocLine)) {
    if (meta.newLine !== null) {
      start = Math.min(start, meta.newLine)
      end = Math.max(end, meta.newLine)
    }
  }
  return start === Infinity ? null : { start, end }
}

// formatLineRef renders a range for a file reference: a single-line selection
// reads as "19", not "19-19".
export function formatLineRef(r: NewLineRange): string {
  return r.start === r.end ? `${r.start}` : `${r.start}-${r.end}`
}

/** Which file a review thread hangs off: GitHub's RIGHT is the new file, LEFT a
 * line the branch deleted. */
export type DiffSide = "RIGHT" | "LEFT"

// docLineAt maps a file line back to the document line rendering it — the
// inverse of the gutter, and what anchors a review thread to the diff it is
// about. A line the diff does not show has no anchor here: GitHub numbers
// against the whole file, while the document holds only the hunks, so a thread
// on untouched code (or one the branch has since rewritten) yields null and is
// rendered away from the file instead of on the wrong line.
export function docLineAt(lineMeta: DiffLine[], side: DiffSide, line: number): number | null {
  if (line <= 0) {
    return null
  }
  const index = lineMeta.findIndex((meta) =>
    side === "LEFT" ? meta.oldLine === line : meta.newLine === line,
  )
  return index === -1 ? null : index + 1
}
