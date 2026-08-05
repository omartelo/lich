import { useMemo } from "react"
import type { RefObject } from "react"
import type { Extension } from "@codemirror/state"
import type { EditorView } from "@codemirror/view"
import { lineNumbers } from "@codemirror/view"
import { blockWidgetGutter, type DocLineSelection } from "@/lib/codemirror"
import { useCodeMirrorView } from "@/lib/use-codemirror-view"

export interface FileEditor {
  containerRef: RefObject<HTMLDivElement>
  /** 1-based file line span of the current selection, or null when empty. */
  getSelectedLines: () => DocLineSelection | null
  /** The live view, for a caller that dispatches into it. */
  view: EditorView | null
}

// useFileEditor is the file-browser preview's read-only CodeMirror view: the
// shared editor plus a plain line gutter. Unlike the diff beside it there is no
// interleaving of added and deleted lines, so that gutter is identity — doc
// line N is file line N, and a selection maps straight to file line numbers
// with no remap.
//
// `extra` is laid on top for the gap the comment box opens between two lines.
// It has to be a stable reference: it rides the view's identity, so a new one
// on every render would rebuild the editor on every render.
export function useFileEditor(text: string, filename: string, extra?: Extension): FileEditor {
  const source = useMemo(
    () => ({ text, extensions: [lineNumbers(), blockWidgetGutter(), ...(extra ? [extra] : [])] }),
    [text, extra],
  )
  return useCodeMirrorView(source, filename)
}
