import { useMemo } from "react"
import type { RefObject } from "react"
import { buildLineDecorations, diffGutter, type DocLineSelection } from "@/lib/codemirror"
import { useCodeMirrorView } from "@/lib/use-codemirror-view"
import type { FileDoc } from "@/lib/git/diff"

export interface DiffEditor {
  containerRef: RefObject<HTMLDivElement>
  /** 1-based doc line span of the current selection, or null when empty. */
  getSelectedDocLines: () => DocLineSelection | null
}

// useDiffEditor is the diff card's read-only CodeMirror view: the shared editor
// plus the two extensions that make it a diff — the +/- gutter and the per-line
// background. Its selection lands on *doc* lines, which are the diff's own
// interleaving of added and deleted, not the file's numbering; mapping them
// back is newLineRange's job at the call site.
export function useDiffEditor(doc: FileDoc, filename: string): DiffEditor {
  const source = useMemo(
    () => ({
      text: doc.text,
      extensions: [diffGutter(doc.lineMeta), buildLineDecorations(doc.lineMeta)],
    }),
    [doc],
  )
  const { containerRef, getSelectedLines } = useCodeMirrorView(source, filename)
  return { containerRef, getSelectedDocLines: getSelectedLines }
}
