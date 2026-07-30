// Block widgets that open a gap between two lines of a diff, so a review thread
// renders where its code is instead of under the whole file.
//
// The widget owns nothing but the gap: it creates an empty div and hands it
// back, and React fills it through a portal. That split is deliberate — a
// comment box holds text somebody is typing, and CodeMirror rebuilds its DOM
// whenever it feels like it. Keeping the content in React (and the draft itself
// in the pending-review store) means a rebuild moves the box, never empties it.
//
// Slots arrive through a StateEffect rather than through the view's extensions:
// resolving a thread or adding a comment must not rebuild the editor, which is
// what replacing an extension does.

import {
  RangeSetBuilder,
  StateEffect,
  StateField,
  type Extension,
  type Text,
} from "@codemirror/state"
import { Decoration, EditorView, WidgetType, type DecorationSet } from "@codemirror/view"

/** Where one gap goes: a stable key, and the 1-based document line it opens after. */
export interface ThreadSlot {
  key: string
  docLine: number
}

/** The gaps a view currently holds, key → the element React renders into. */
export type SlotElements = ReadonlyMap<string, HTMLElement>

const setSlots = StateEffect.define<ThreadSlot[]>()

type Register = (key: string, element: HTMLElement | null) => void

// SlotWidget is compared by key alone: two decorations with the same key are the
// same gap, so CodeMirror keeps the element it already made — and with it the
// portal, and with the portal whatever was half-typed inside.
class SlotWidget extends WidgetType {
  constructor(
    private readonly key: string,
    private readonly register: Register,
  ) {
    super()
  }

  eq(other: SlotWidget): boolean {
    return other.key === this.key
  }

  toDOM(): HTMLElement {
    const element = document.createElement("div")
    element.className = "cm-thread-slot"
    this.register(this.key, element)
    return element
  }

  destroy(): void {
    this.register(this.key, null)
  }

  // The gap is React's surface: its clicks, selections and keystrokes belong to
  // the comment box inside it, not to the editor around it.
  ignoreEvent(): boolean {
    return true
  }
}

export interface ThreadSlots {
  extension: Extension
  /** Replace the view's gaps. Cheap to call with the same list — an unchanged
   * key keeps its element. */
  update(view: EditorView, slots: ThreadSlot[]): void
}

// threadSlots builds one view's slot extension. onChange fires whenever the set
// of live elements changes, which is React's cue to re-render its portals.
//
// The callback is deferred to a microtask because it runs from inside
// CodeMirror's own DOM update, and a React state change dispatched from there
// would re-enter the update it is still in the middle of.
export function threadSlots(onChange: (elements: SlotElements) => void): ThreadSlots {
  const elements = new Map<string, HTMLElement>()
  let scheduled = false

  const announce = (): void => {
    if (scheduled) {
      return
    }
    scheduled = true
    queueMicrotask(() => {
      scheduled = false
      onChange(new Map(elements))
    })
  }

  const register: Register = (key, element) => {
    if (element) {
      elements.set(key, element)
    } else {
      elements.delete(key)
    }
    announce()
  }

  const field = StateField.define<DecorationSet>({
    create: () => Decoration.none,
    update(current, transaction) {
      for (const effect of transaction.effects) {
        if (effect.is(setSlots)) {
          return buildSlots(transaction.state.doc, effect.value, register)
        }
      }
      return current
    },
    provide: (field) => EditorView.decorations.from(field),
  })

  return {
    extension: field,
    update: (view, slots) => view.dispatch({ effects: setSlots.of(slots) }),
  }
}

// buildSlots turns the requested gaps into decorations, anchored at the end of
// their line — decorations address character offsets, not line numbers. A
// RangeSet has to be built in document order, and a slot past the end of the
// document (a thread on a line this diff no longer renders) is dropped rather
// than clamped onto an unrelated line.
function buildSlots(doc: Text, slots: ThreadSlot[], register: Register): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>()
  const ordered = [...slots]
    .filter((slot) => slot.docLine >= 1 && slot.docLine <= doc.lines)
    .sort((a, b) => a.docLine - b.docLine || a.key.localeCompare(b.key))
  for (const slot of ordered) {
    const at = doc.line(slot.docLine).to
    builder.add(
      at,
      at,
      Decoration.widget({ widget: new SlotWidget(slot.key, register), block: true, side: 1 }),
    )
  }
  return builder.finish()
}
