// The inline comment composer inside a diff: the box that opens under the lines
// a review comment is about, so the note is written where it is aimed rather
// than in a dialog over the middle of the screen.
//
// A CodeMirror block widget, not a React tree: the editor owns this DOM, and
// mounting a portal into a widget would tie React's lifecycle to the view's.
// The widget is created once per draft (eq keeps it alive across unrelated
// state changes) — recreating it would throw away what was typed.

import { StateEffect, StateField, type Extension } from "@codemirror/state"
import { Decoration, EditorView, WidgetType, type DecorationSet } from "@codemirror/view"

export interface CommentDraft {
  /** Doc line the composer hangs under — the last line of the selection. */
  line: number
  /** The new-file range the comment is filed against: "19" or "12-30". */
  lines: string
}

/** Open the composer on a draft, or close it with null. */
export const setCommentDraft = StateEffect.define<CommentDraft | null>()

const ROOT_CLASS = "flex flex-col gap-1 px-2 py-1.5 font-sans"
const LABEL_CLASS = "font-mono text-[0.6875rem] text-muted-foreground"
const FIELD_CLASS =
  "min-h-14 w-full resize-y rounded-md border border-input bg-transparent px-2 py-1 text-xs outline-none transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
const HINT_CLASS = "text-right text-[0.625rem] text-muted-foreground"

class ComposerWidget extends WidgetType {
  constructor(
    readonly draft: CommentDraft,
    readonly submit: (lines: string, text: string) => void,
  ) {
    super()
  }

  eq(other: ComposerWidget): boolean {
    return other.draft.line === this.draft.line && other.draft.lines === this.draft.lines
  }

  toDOM(view: EditorView): HTMLElement {
    const root = document.createElement("div")
    // The editor's content is contenteditable; the composer must not be, or
    // typing in it would run through CodeMirror's own input handling.
    root.contentEditable = "false"
    root.className = ROOT_CLASS

    const label = document.createElement("div")
    label.className = LABEL_CLASS
    label.textContent = `comment · ${this.draft.lines}`

    const field = document.createElement("textarea")
    field.className = FIELD_CLASS
    field.rows = 2
    field.placeholder = "What should change here?"

    const hint = document.createElement("div")
    hint.className = HINT_CLASS
    hint.textContent = "Esc to cancel · ⏎ to add · ⇧⏎ for a new line"

    const close = () => view.dispatch({ effects: setCommentDraft.of(null) })

    field.addEventListener("keydown", (event) => {
      // Every key stops here: the diff's own shortcuts must not read what is
      // being typed into the note.
      event.stopPropagation()
      if (event.key === "Escape") {
        event.preventDefault()
        close()
        view.focus()
        return
      }
      if (event.key === "Enter" && !event.shiftKey) {
        event.preventDefault()
        const text = field.value.trim()
        if (text !== "") {
          this.submit(this.draft.lines, text)
        }
        close()
      }
    })

    root.append(label, field, hint)
    // The node is in the document only after CodeMirror inserts it.
    requestAnimationFrame(() => field.focus())
    return root
  }

  ignoreEvent(): boolean {
    return true
  }
}

// commentComposer holds at most one open composer per editor — one file's diff.
// Its submit callback is captured once, so callers hand over a stable function
// that reads the current handler rather than a new closure per render.
export function commentComposer(submit: (lines: string, text: string) => void): Extension {
  return StateField.define<DecorationSet>({
    create: () => Decoration.none,
    update(decorations, tr) {
      for (const effect of tr.effects) {
        if (!effect.is(setCommentDraft)) {
          continue
        }
        if (!effect.value) {
          return Decoration.none
        }
        const line = tr.state.doc.line(Math.min(effect.value.line, tr.state.doc.lines))
        const widget = Decoration.widget({
          widget: new ComposerWidget(effect.value, submit),
          block: true,
          side: 1,
        })
        return Decoration.set([widget.range(line.to)])
      }
      return decorations.map(tr.changes)
    },
    provide: (field) => EditorView.decorations.from(field),
  })
}
