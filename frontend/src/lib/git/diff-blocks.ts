import type { RevertLine } from "@/lib/api-types"
import type { DiffLine } from "./diff"

// A change block is one run of deleted and added lines with no unchanged line
// between them: the unit the diff's hover Revert undoes in one click.

export interface ChangeBlock {
  /** 1-based document line the run starts on, which is also its identity. */
  from: number
  /** 1-based document line the run ends on, inclusive. */
  to: number
}

const isChange = (line: DiffLine): boolean => line.kind === "add" || line.kind === "del"

// changeBlocks finds every run in document order.
export function changeBlocks(lineMeta: DiffLine[]): ChangeBlock[] {
  const blocks: ChangeBlock[] = []
  for (const [index, line] of lineMeta.entries()) {
    if (!isChange(line)) {
      continue
    }
    const last = blocks[blocks.length - 1]
    if (last && last.to === index) {
      last.to = index + 1
    } else {
      blocks.push({ from: index + 1, to: index + 1 })
    }
  }
  return blocks
}

// blockOfLine lays the blocks out per document line: the block's `from` for a
// line inside one, null for anything else. What a hover resolves against.
export function blockOfLine(lineMeta: DiffLine[]): (number | null)[] {
  const byLine: (number | null)[] = lineMeta.map(() => null)
  for (const block of changeBlocks(lineMeta)) {
    byLine.fill(block.from, block.from - 1, block.to)
  }
  return byLine
}

// revertLinesIn lists the changed lines inside a document span as the backend
// names them: a deletion by its old number, an addition by its new one, each
// with its text so a stale screen is refused rather than obeyed.
export function revertLinesIn(lineMeta: DiffLine[], from: number, to: number): RevertLine[] {
  const lines: RevertLine[] = []
  for (const line of lineMeta.slice(from - 1, to)) {
    if (line.kind === "del" && line.oldLine !== null) {
      lines.push({ side: "old", line: line.oldLine, text: line.text })
    } else if (line.kind === "add" && line.newLine !== null) {
      lines.push({ side: "new", line: line.newLine, text: line.text })
    }
  }
  return lines
}

// revertStat counts a revert the way DiffStat draws a diff.
export function revertStat(lines: RevertLine[]): { added: number; deleted: number } {
  const added = lines.filter((line) => line.side === "new").length
  return { added, deleted: lines.length - added }
}
