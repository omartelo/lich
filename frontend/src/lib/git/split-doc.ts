import { blockOfLine } from "./diff-blocks"
import type { DiffGap, DiffLine, FileDoc } from "./diff"

// The side-by-side layout is two documents, one per side, built so that row N
// of one is row N of the other: an unchanged line sits on both, a change block
// pairs its deletions with its additions by position, and whichever side runs
// out first is padded with filler rows. Equal row counts are what let two
// editors that never wrap stay level without measuring anything.

export interface SplitDoc {
  /** HEAD's side: deletions, unchanged lines, separators and fillers. Its gaps
   * are empty: the expanders live on the new side alone. */
  left: FileDoc
  /** The new side: additions, unchanged lines, separators and fillers. */
  right: FileDoc
  /** Per row, the line review threads anchor against: the old number from the
   * left, the new one from the right. */
  anchors: DiffLine[]
  /** Per row, the change block it belongs to (see diff-blocks), or null. */
  blocks: (number | null)[]
}

const filler: DiffLine = { kind: "filler", text: "", oldLine: null, newLine: null }

// buildSplitDoc lays a unified document out as two sides.
export function buildSplitDoc(doc: FileDoc): SplitDoc {
  const byLine = blockOfLine(doc.lineMeta)
  const left: DiffLine[] = []
  const right: DiffLine[] = []
  const blocks: (number | null)[] = []
  // Unified document line → row, for the gaps, which are addressed by line.
  const rowOf = new Map<number, number>()
  const meta = doc.lineMeta

  for (let index = 0; index < meta.length; ) {
    const block = byLine[index]
    if (block === null) {
      rowOf.set(index + 1, left.length + 1)
      left.push(meta[index])
      right.push(meta[index])
      blocks.push(null)
      index++
      continue
    }
    const run: DiffLine[] = []
    while (index < meta.length && byLine[index] === block) {
      run.push(meta[index])
      index++
    }
    const dels = run.filter((line) => line.kind === "del")
    const adds = run.filter((line) => line.kind === "add")
    for (let pair = 0; pair < Math.max(dels.length, adds.length); pair++) {
      left.push(dels[pair] ?? filler)
      right.push(adds[pair] ?? filler)
      blocks.push(block)
    }
  }

  const gaps = doc.gaps.map((gap): DiffGap => ({ ...gap, docLine: rowOf.get(gap.docLine) ?? 0 }))
  return {
    left: sideDoc(left, []),
    right: sideDoc(right, gaps),
    anchors: left.map((line, row) => ({
      kind: right[row].kind === "filler" ? line.kind : right[row].kind,
      text: "",
      oldLine: line.oldLine,
      newLine: right[row].newLine,
    })),
    blocks,
  }
}

function sideDoc(lineMeta: DiffLine[], gaps: DiffGap[]): FileDoc {
  return { text: lineMeta.map((line) => line.text).join("\n"), lineMeta, gaps }
}
