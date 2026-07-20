import {useEffect, useRef} from "react"
import type {RefObject} from "react"
import {EditorState} from "@codemirror/state"
import {EditorView, lineNumbers} from "@codemirror/view"
import {
  loadLanguage,
  readOnlyCodeExtensions,
  selectedDocLines,
  type DocLineSelection,
} from "@/lib/codemirror"

export interface FileEditor {
  containerRef: RefObject<HTMLDivElement>
  /** 1-based file line span of the current selection, or null when empty. */
  getSelectedLines: () => DocLineSelection | null
}

// useFileEditor owns one read-only CodeMirror view over a whole file's text —
// the file-browser preview. Unlike the diff viewer there is no interleaving of
// added/deleted lines, so the plain line gutter is identity: doc line N is
// file line N, and a selection maps straight to file line numbers with no
// remap. Created on mount, destroyed on unmount or when the text is replaced.
export function useFileEditor(text: string, filename: string): FileEditor {
  const containerRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)

  useEffect(() => {
    const parent = containerRef.current
    if (!parent) {
      return
    }
    const view = new EditorView({
      state: EditorState.create({
        doc: text,
        extensions: [...readOnlyCodeExtensions(), lineNumbers()],
      }),
      parent,
    })
    viewRef.current = view
    loadLanguage(view, filename, () => viewRef.current === view)
    return () => {
      viewRef.current = null
      view.destroy()
    }
  }, [text, filename])

  const getSelectedLines = (): DocLineSelection | null =>
    viewRef.current ? selectedDocLines(viewRef.current) : null

  return {containerRef, getSelectedLines}
}
