import { useMemo, useRef } from "react"
import type { RefObject } from "react"
import { buildLineDecorations, diffGutter, type DocLineSelection } from "@/lib/codemirror"
import { commentComposer, setCommentDraft, type CommentDraft } from "@/lib/codemirror-comment"
import { useCodeMirrorView } from "@/lib/use-codemirror-view"
import type { FileDoc } from "@/lib/git/diff"

export interface DiffEditor {
  containerRef: RefObject<HTMLDivElement>
  /** 1-based doc line span of the current selection, or null when empty. */
  getSelectedDocLines: () => DocLineSelection | null
  /** Open the inline comment composer under a draft's line. */
  openComment: (draft: CommentDraft) => void
}

// useDiffEditor is the diff card's read-only CodeMirror view: the shared editor
// plus the extensions that make it a diff — the +/- gutter, the per-line
// background, and the comment composer that opens between the lines.
//
// Its selection lands on *doc* lines, which are the diff's own interleaving of
// added and deleted, not the file's numbering; mapping them back is
// newLineRange's job at the call site.
export function useDiffEditor(
  doc: FileDoc,
  filename: string,
  onComment?: (lines: string, text: string) => void,
): DiffEditor {
  // The extensions are built once per document, so the composer cannot capture
  // this render's handler; it reads the current one off the ref instead.
  const submit = useRef(onComment)
  submit.current = onComment

  const source = useMemo(
    () => ({
      text: doc.text,
      extensions: [
        diffGutter(doc.lineMeta),
        buildLineDecorations(doc.lineMeta),
        commentComposer((lines, text) => submit.current?.(lines, text)),
      ],
    }),
    [doc],
  )
  const { containerRef, getSelectedLines, dispatchEffects } = useCodeMirrorView(source, filename)

  return {
    containerRef,
    getSelectedDocLines: getSelectedLines,
    openComment: (draft) => dispatchEffects([setCommentDraft.of(draft)]),
  }
}
