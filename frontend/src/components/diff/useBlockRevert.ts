import { useCallback, useEffect, useMemo, useRef } from "react"
import type { RevertLine } from "@/lib/api-types"
import { blockHover, type BlockHover } from "@/lib/codemirror-revert"
import { changeBlocks, revertLinesIn } from "@/lib/git/diff-blocks"
import type { FileDoc } from "@/lib/git/diff"

export interface BlockRevert {
  /** Whether this diff offers reverting at all. */
  revertable: boolean
  hover: BlockHover
  /** Stable: rides the editor's identity through the revert layer. */
  revertBlock: (block: number) => void
  revertLines: (lines: RevertLine[]) => void
}

// useBlockRevert resolves a block the hover names back to its changed lines, in
// the unified document either layout is built from. The prop is read through a
// ref so the handler stays stable while the panel re-renders around it.
export function useBlockRevert(
  doc: FileDoc,
  onRevertLines?: (lines: RevertLine[]) => void,
): BlockRevert {
  const latest = useRef(onRevertLines)
  useEffect(() => {
    latest.current = onRevertLines
  })
  const hover = useMemo(blockHover, [])
  const revertBlock = useCallback(
    (block: number) => {
      const found = changeBlocks(doc.lineMeta).find((candidate) => candidate.from === block)
      if (found) {
        latest.current?.(revertLinesIn(doc.lineMeta, found.from, found.to))
      }
    },
    [doc],
  )
  const revertLines = useCallback((lines: RevertLine[]) => latest.current?.(lines), [])
  return { revertable: onRevertLines !== undefined, hover, revertBlock, revertLines }
}
