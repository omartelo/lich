import type { DiffFile, DiffHunk, DiffLine } from "./diff"

// hideWhitespace drops the changes that only moved whitespace, on the parsed
// diff rather than with `git diff -w`: the pull request diff comes from
// `gh pr diff`, which has no such flag, and every panel has to agree on what
// "hidden" means.
//
// The unit is the whole change block, never a single line pair. A block whose
// deletions and additions pair up one for one, each pair equal once whitespace
// is ignored, turns into unchanged lines showing the file as it is now; any
// other block is left whole. Folding pairs out of a mixed block would leave its
// real change revertable on its own, and reverting that one line puts HEAD's
// indentation back into a block the file has since re-indented.
//
// A hunk left with no change is dropped, which is what git -w does too: the
// unchanged lines either side of it become part of a wider gap, pulled in by
// the expanders like any other. The counts are recounted from what is left.
export function hideWhitespace(file: DiffFile): DiffFile {
  if (file.binary) {
    return file
  }
  const hunks = file.hunks.map(foldHunk).filter((hunk) => hunk.lines.some(isChange))
  const lines = hunks.flatMap((hunk) => hunk.lines)
  return {
    ...file,
    hunks,
    added: lines.filter((line) => line.kind === "add").length,
    deleted: lines.filter((line) => line.kind === "del").length,
  }
}

/** True for a file every change of which was whitespace. Its card says so
 * instead of drawing an empty body. */
export function whitespaceOnly(shown: DiffFile, original: DiffFile): boolean {
  return shown.hunks.length === 0 && original.hunks.length > 0
}

function foldHunk(hunk: DiffHunk): DiffHunk {
  const lines: DiffLine[] = []
  let block: DiffLine[] = []
  const flush = (): void => {
    lines.push(...foldBlock(block))
    block = []
  }
  for (const line of hunk.lines) {
    if (isChange(line)) {
      block.push(line)
      continue
    }
    flush()
    lines.push(line)
  }
  flush()
  return { ...hunk, lines }
}

// foldBlock answers the block as unchanged lines when it only moved whitespace,
// and as itself otherwise. git prints a block's deletions before its additions,
// so the i-th of each is the pair.
function foldBlock(block: DiffLine[]): DiffLine[] {
  const deleted = block.filter((line) => line.kind === "del")
  const added = block.filter((line) => line.kind === "add")
  const onlyWhitespace =
    deleted.length === added.length &&
    deleted.every((line, index) => sameIgnoringWhitespace(line.text, added[index].text))
  if (block.length === 0 || !onlyWhitespace) {
    return block
  }
  return added.map((line, index) => ({
    kind: "context" as const,
    text: ` ${line.text.slice(1)}`,
    oldLine: deleted[index].oldLine,
    newLine: line.newLine,
  }))
}

const isChange = (line: DiffLine): boolean => line.kind === "add" || line.kind === "del"

// Compared past the +/- prefix parseDiff keeps on each line.
const sameIgnoringWhitespace = (deleted: string, added: string): boolean =>
  deleted.slice(1).replace(/\s+/g, "") === added.slice(1).replace(/\s+/g, "")
