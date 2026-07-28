import { useMemo } from "react"
import type { RefObject } from "react"
import { lineNumbers } from "@codemirror/view"
import type { DocLineSelection } from "@/lib/codemirror"
import { useCodeMirrorView } from "@/lib/use-codemirror-view"

export interface FileEditor {
  containerRef: RefObject<HTMLDivElement>
  /** 1-based file line span of the current selection, or null when empty. */
  getSelectedLines: () => DocLineSelection | null
}

// useFileEditor is the file-browser preview's read-only CodeMirror view: the
// shared editor plus a plain line gutter. Unlike the diff beside it there is no
// interleaving of added and deleted lines, so that gutter is identity — doc
// line N is file line N, and a selection maps straight to file line numbers
// with no remap.
export function useFileEditor(text: string, filename: string): FileEditor {
  const source = useMemo(() => ({ text, extensions: [lineNumbers()] }), [text])
  return useCodeMirrorView(source, filename)
}
