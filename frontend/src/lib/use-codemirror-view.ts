import { useEffect, useRef, useState } from "react"
import type { RefObject } from "react"
import type { Extension } from "@codemirror/state"
import { EditorState } from "@codemirror/state"
import { EditorView } from "@codemirror/view"
import { loadLanguage, readOnlyCodeExtensions, selectedDocLines } from "@/lib/codemirror"
import type { DocLineSelection } from "@/lib/codemirror"

// What a view is built from. The two together are the view's identity: a new
// object rebuilds it, so callers hand over a memoised one and the editor
// survives every render that changed neither.
export interface CodeMirrorSource {
  text: string
  /** Laid on top of the shared read-only base — a diff's gutter and line
   * decorations, a plain file's line numbers. */
  extensions: Extension[]
}

export interface CodeMirrorView {
  containerRef: RefObject<HTMLDivElement>
  /** 1-based doc line span of the current selection, or null when empty. */
  getSelectedLines: () => DocLineSelection | null
  /** The live view, or null before it is built. State rather than a ref: a
   * caller that dispatches into the view (the diff's thread slots) has to be
   * told when there is one to dispatch to. */
  view: EditorView | null
}

// useCodeMirrorView owns one read-only CodeMirror view for its container:
// created on mount, destroyed on unmount or when the source is replaced by a
// refresh. The diff cards and the file-browser preview are the same editor —
// they differ only in the extensions they layer on and in what a selection's
// doc lines mean afterwards, which is the wrapping hook's business, not this
// one's.
export function useCodeMirrorView(source: CodeMirrorSource, filename: string): CodeMirrorView {
  const containerRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const [view, setView] = useState<EditorView | null>(null)

  useEffect(() => {
    const parent = containerRef.current
    if (!parent) {
      return
    }
    const view = new EditorView({
      state: EditorState.create({
        doc: source.text,
        extensions: [...readOnlyCodeExtensions(), ...source.extensions],
      }),
      parent,
    })
    viewRef.current = view
    setView(view)
    // Async: the language may still be loading when the view is replaced, so
    // the callback checks it is still the live one before touching it.
    loadLanguage(view, filename, () => viewRef.current === view)
    return () => {
      viewRef.current = null
      setView(null)
      view.destroy()
    }
  }, [source, filename])

  const getSelectedLines = (): DocLineSelection | null =>
    viewRef.current ? selectedDocLines(viewRef.current) : null

  return { containerRef, getSelectedLines, view }
}
