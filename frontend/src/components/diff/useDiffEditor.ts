import { useMemo } from "react"
import type { RefObject } from "react"
import type { Extension } from "@codemirror/state"
import type { EditorView } from "@codemirror/view"
import { buildLineDecorations, diffGutter, type DocLineSelection } from "@/lib/codemirror"
import { useCodeMirrorView } from "@/lib/use-codemirror-view"
import type { FileDoc } from "@/lib/git/diff"

export interface DiffEditor {
  containerRef: RefObject<HTMLDivElement>
  /** 1-based doc line span of the current selection, or null when empty. */
  getSelectedDocLines: () => DocLineSelection | null
  /** The live view, for a caller that dispatches into it. */
  view: EditorView | null
}

// useDiffEditor is the diff card's read-only CodeMirror view: the shared editor
// plus the two extensions that make it a diff — the +/- gutter and the per-line
// background. Its selection lands on *doc* lines, which are the diff's own
// interleaving of added and deleted, not the file's numbering; mapping them
// back is newLineRange's job at the call site.
//
// `extra` is laid on top for a caller that needs more of the editor — the pull
// request's review threads open their gaps through it. It has to be a stable
// reference: it rides the view's identity, so a new one on every render would
// rebuild the editor on every render.
export function useDiffEditor(doc: FileDoc, filename: string, extra?: Extension): DiffEditor {
  const source = useMemo(
    () => ({
      text: doc.text,
      extensions: [
        diffGutter(doc.lineMeta),
        buildLineDecorations(doc.lineMeta),
        ...(extra ? [extra] : []),
      ],
    }),
    [doc, extra],
  )
  const { containerRef, getSelectedLines, view } = useCodeMirrorView(source, filename)
  return { containerRef, getSelectedDocLines: getSelectedLines, view }
}
