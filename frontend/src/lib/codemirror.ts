// Shared CodeMirror 6 building blocks. The diff viewer is the first consumer;
// future editors should grow their own extension sets here instead of
// configuring views inline.

import { EditorState, RangeSetBuilder, StateEffect, type Extension } from "@codemirror/state"
import { LanguageDescription, syntaxHighlighting } from "@codemirror/language"
import { languages } from "@codemirror/language-data"
import { classHighlighter } from "@lezer/highlight"
import {
  Decoration,
  EditorView,
  GutterMarker,
  gutterLineClass,
  lineNumberWidgetMarker,
  lineNumbers,
} from "@codemirror/view"
import { gutterNumber, type DiffLine } from "@/lib/git/diff"

// diffTheme styles the editor with the app's CSS variables, so the `.dark`
// class on <html> restyles every view without any JS synchronization.
const diffTheme = EditorView.theme({
  "&": {
    backgroundColor: "transparent",
    color: "var(--foreground)",
    fontSize: "12px",
  },
  ".cm-content": {
    fontFamily: "'FiraCode Nerd Font Mono', monospace",
    padding: "0",
  },
  ".cm-line": { padding: "0 0.5rem" },
  "&.cm-focused": { outline: "none" },
  ".cm-gutters": {
    backgroundColor: "transparent",
    border: "none",
    color: "var(--muted-foreground)",
    fontFamily: "'FiraCode Nerd Font Mono', monospace",
  },
  ".cm-lineNumbers .cm-gutterElement": {
    padding: "0 0.75rem 0 0.5rem",
    minWidth: "2.25rem",
  },
  // The gap a review thread renders into (codemirror-threads.ts). Everything
  // here is undoing the editor: the content inside is prose in a React tree, not
  // code, so it takes the app's font and wraps like text rather than inheriting
  // the monospace pre-wrap of the lines around it.
  //
  // The breathing room is padding here and never a margin inside: CodeMirror
  // sizes a block widget from the element's own box, which a child's margin
  // sits outside of. Every pixel of margin in there is a pixel the gutter does
  // not know about, and it drifts the line numbers below by that much.
  ".cm-thread-slot": {
    fontFamily: "var(--font-sans)",
    fontSize: "0.8125rem",
    lineHeight: "1.5",
    whiteSpace: "normal",
    tabSize: "unset",
    padding: "0.375rem 0.5rem",
    cursor: "auto",
    userSelect: "text",
  },
})

// readOnlyCodeExtensions is the base set for any read-only code viewer — the
// diff panel and the file preview both build on it. EditorState.readOnly (not
// EditorView.editable) blocks edits while keeping the DOM contenteditable,
// which CodeMirror needs to track text selection. classHighlighter emits tok-*
// classes; the palette lives in index.css.
export function readOnlyCodeExtensions(): Extension[] {
  return [
    EditorState.readOnly.of(true),
    EditorView.lineWrapping,
    syntaxHighlighting(classHighlighter),
    diffTheme,
  ]
}

// DocLineSelection is a 1-based inclusive span of document lines.
export interface DocLineSelection {
  from: number
  to: number
}

// selectedDocLines resolves a view's current selection to the doc line span it
// covers, or null when the selection is empty. Shared by the diff and file
// viewers; the diff viewer then remaps these doc lines to new-file lines, while
// the file viewer uses them directly (doc line === file line).
export function selectedDocLines(view: EditorView): DocLineSelection | null {
  const { main } = view.state.selection
  if (main.empty) {
    return null
  }
  const from = view.state.doc.lineAt(main.from).number
  const toLine = view.state.doc.lineAt(main.to)
  // A selection ending exactly at a line's start doesn't include that line.
  const to = main.to === toLine.from ? toLine.number - 1 : toLine.number
  return to < from ? null : { from, to }
}

// loadLanguage appends the file's language support once its lazy chunk
// resolves. The view may have been destroyed meanwhile (collapse, refresh) —
// the isAlive guard skips the dispatch then. Files without a matching parser
// stay unhighlighted. Shared by the diff and file viewers.
export function loadLanguage(view: EditorView, filename: string, isAlive: () => boolean): void {
  const description = LanguageDescription.matchFilename(languages, filename)
  if (!description) {
    return
  }
  description
    .load()
    .then((support) => {
      if (isAlive()) {
        view.dispatch({ effects: StateEffect.appendConfig.of(support.extension) })
      }
    })
    .catch(() => {})
}

const lineClasses: Partial<Record<DiffLine["kind"], Decoration>> = {
  add: Decoration.line({ class: "cm-diff-add" }),
  del: Decoration.line({ class: "cm-diff-del" }),
  meta: Decoration.line({ class: "cm-diff-sep" }),
}

// gutterLineClass markers carry only an elementClass — the colored strip and
// tinted background of changed lines' gutter cells.
class LineClassMarker extends GutterMarker {
  constructor(readonly elementClass: string) {
    super()
  }
}

const gutterMarkers: Partial<Record<DiffLine["kind"], LineClassMarker>> = {
  add: new LineClassMarker("cm-diff-gutter-add"),
  del: new LineClassMarker("cm-diff-gutter-del"),
}

// buildLineDecorations colors added/deleted lines and hunk separators, in the
// content area and in the gutter. The doc of a diff view never changes (a
// refresh replaces the whole view), so static RangeSets are enough — no
// ViewPlugin or StateField needed.
export function buildLineDecorations(lineMeta: DiffLine[]): Extension {
  const lines = new RangeSetBuilder<Decoration>()
  const gutters = new RangeSetBuilder<GutterMarker>()
  let pos = 0
  for (const meta of lineMeta) {
    const decoration = lineClasses[meta.kind]
    if (decoration) {
      lines.add(pos, pos, decoration)
    }
    const marker = gutterMarkers[meta.kind]
    if (marker) {
      gutters.add(pos, pos, marker)
    }
    pos += meta.text.length + 1 // +1 for the newline
  }
  return [EditorView.decorations.of(lines.finish()), gutterLineClass.of(gutters.finish())]
}

// A block widget — the gap a review thread or a comment box opens between two
// lines — is a block in the content but not a line, and the gutter draws
// nothing for a widget unless it is given a marker for it. Nothing at all is
// the bug: the gutter ends up one cell short of the content's blocks, so every
// number below the widget sits beside the line above its own. An empty marker
// buys the widget a cell of its own height and nothing else, which is what
// keeps the numbers level with their code.
class BlockSpacer extends GutterMarker {
  toDOM(): Node {
    return document.createTextNode("")
  }
}

const blockSpacer = new BlockSpacer()

// diffGutter is the single-column line gutter: numbers come from lineMeta
// (old-file for deletions, new-file otherwise), not from doc line numbers.
// lineNumbers' internal spacer probes formatNumber with out-of-range numbers
// to size the gutter; those get the widest real number so it never collapses.
export function diffGutter(lineMeta: DiffLine[]): Extension {
  const widest = String(Math.max(1, ...lineMeta.map((meta) => meta.newLine ?? meta.oldLine ?? 0)))
  return [
    lineNumbers({
      formatNumber: (lineNo) => {
        const meta = lineMeta[lineNo - 1]
        return meta ? gutterNumber(meta) : widest
      },
    }),
    lineNumberWidgetMarker.of(() => blockSpacer),
  ]
}
