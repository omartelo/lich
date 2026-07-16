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
  return path.startsWith("a/") || path.startsWith("b/")
    ? path.slice(2)
    : path
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
    hunk.lines.push({kind: "add", text: line, oldLine: null, newLine: counters.new})
    counters.new += 1
    file.added += 1
  } else if (line.startsWith("-")) {
    hunk.lines.push({kind: "del", text: line, oldLine: counters.old, newLine: null})
    counters.old += 1
    file.deleted += 1
  } else if (line.startsWith("\\")) {
    hunk.lines.push({kind: "meta", text: line, oldLine: null, newLine: null})
  } else {
    hunk.lines.push({kind: "context", text: line, oldLine: counters.old, newLine: counters.new})
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
  const counters = {old: 0, new: 0}

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
}

const hunkSeparator: DiffLine = {kind: "meta", text: "", oldLine: null, newLine: null}

export function buildFileDoc(file: DiffFile): FileDoc {
  const lines: string[] = []
  const lineMeta: DiffLine[] = []
  for (const [index, hunk] of file.hunks.entries()) {
    if (index > 0) {
      lines.push("")
      lineMeta.push(hunkSeparator)
    }
    for (const line of hunk.lines) {
      if (line.kind === "meta") {
        continue
      }
      const text = line.text.slice(1)
      lines.push(text)
      // lineMeta's text must equal the doc's, so decoration position math
      // (pos += text.length + 1) stays in step.
      lineMeta.push({...line, text})
    }
  }
  return {text: lines.join("\n"), lineMeta}
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
export function gutterNumber(line: DiffLine): string {
  if (line.kind === "del") {
    return String(line.oldLine)
  }
  return line.newLine === null ? "" : String(line.newLine)
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
  return start === Infinity ? null : {start, end}
}

// formatLineRef renders a range for a file reference: a single-line selection
// reads as "19", not "19-19".
export function formatLineRef(r: NewLineRange): string {
  return r.start === r.end ? `${r.start}` : `${r.start}-${r.end}`
}
