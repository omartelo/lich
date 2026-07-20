import {useEffect, useRef} from "react"
import type {RefObject} from "react"
import {EditorState} from "@codemirror/state"
import {EditorView} from "@codemirror/view"
import {
  buildLineDecorations,
  diffGutter,
  loadLanguage,
  readOnlyCodeExtensions,
  selectedDocLines,
  type DocLineSelection,
} from "@/lib/codemirror"
import type {FileDoc} from "@/lib/diff"

export interface DiffEditor {
  containerRef: RefObject<HTMLDivElement>
  /** 1-based doc line span of the current selection, or null when empty. */
  getSelectedDocLines: () => DocLineSelection | null
}

// useDiffEditor owns one read-only CodeMirror view: created when the container
// mounts, destroyed on unmount or when the doc is replaced by a refresh.
export function useDiffEditor(doc: FileDoc, filename: string): DiffEditor {
  const containerRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)

  useEffect(() => {
    const parent = containerRef.current
    if (!parent) {
      return
    }
    const view = new EditorView({
      state: EditorState.create({
        doc: doc.text,
        extensions: [
          ...readOnlyCodeExtensions(),
          diffGutter(doc.lineMeta),
          buildLineDecorations(doc.lineMeta),
        ],
      }),
      parent,
    })
    viewRef.current = view
    loadLanguage(view, filename, () => viewRef.current === view)
    return () => {
      viewRef.current = null
      view.destroy()
    }
  }, [doc, filename])

  const getSelectedDocLines = (): DocLineSelection | null =>
    viewRef.current ? selectedDocLines(viewRef.current) : null

  return {containerRef, getSelectedDocLines}
}
