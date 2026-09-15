// The diff's hover Revert: pointing at a change block tints the whole block and
// puts a Revert button on its first row. A side-by-side diff is two editors
// showing one block, so the hover is shared: every view registered with the
// same BlockHover lights the same block, whichever of them the pointer is over.

import {
  RangeSetBuilder,
  StateEffect,
  StateField,
  type Extension,
  type Text,
} from "@codemirror/state"
import {
  Decoration,
  type DecorationSet,
  EditorView,
  ViewPlugin,
  WidgetType,
} from "@codemirror/view"

export interface RevertLayer {
  /** Per document line, the change block it belongs to, or null (diff-blocks). */
  blocks: (number | null)[]
  /** Block → the document line its Revert button sits on. Absent for a view
   * that only mirrors the tint, like the left side of a split diff. */
  buttons?: ReadonlyMap<number, number>
  /** Must be a stable reference: it rides the view's identity. */
  onRevert: (block: number) => void
}

export interface BlockHover {
  /** The layer for one view; every view built from one BlockHover shares its hover. */
  layer(options: RevertLayer): Extension
}

const setHot = StateEffect.define<number | null>()

const hotField = StateField.define<number | null>({
  create: () => null,
  update(current, transaction) {
    for (const effect of transaction.effects) {
      if (effect.is(setHot)) {
        return effect.value
      }
    }
    return current
  },
})

const hotLine = Decoration.line({ class: "diff-block-hot" })

// Lucide's undo-2, drawn by hand: the button lives in CodeMirror's DOM, where
// there is no React to render the icon component into.
const UNDO_ICON =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 5.5 5.5a5.5 5.5 0 0 1-5.5 5.5H11"/></svg>'

class RevertWidget extends WidgetType {
  constructor(
    readonly block: number,
    readonly onRevert: (block: number) => void,
  ) {
    super()
  }

  eq(other: RevertWidget): boolean {
    return other.block === this.block
  }

  toDOM(): HTMLElement {
    const button = document.createElement("button")
    button.type = "button"
    button.className = "cm-diff-revert"
    button.title = "Revert this change"
    button.innerHTML = `${UNDO_ICON}<span>Revert</span>`
    // Pressing it must not move the editor's selection off whatever was selected.
    button.addEventListener("mousedown", (event) => event.preventDefault())
    button.addEventListener("click", () => this.onRevert(this.block))
    return button
  }

  ignoreEvent(): boolean {
    return true
  }
}

// buildRevertDecorations tints the hot block's lines and places its button.
// Exported for the suite: a line carrying both lands a line decoration and a
// widget in one RangeSetBuilder, which throws when they arrive out of order.
export function buildRevertDecorations(
  doc: Text,
  layer: RevertLayer,
  hot: number | null,
): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>()
  if (hot === null) {
    return builder.finish()
  }
  const buttonLine = layer.buttons?.get(hot)
  for (const [index, block] of layer.blocks.entries()) {
    if (block !== hot || index >= doc.lines) {
      continue
    }
    const line = doc.line(index + 1)
    builder.add(line.from, line.from, hotLine)
    if (buttonLine === index + 1) {
      builder.add(
        line.to,
        line.to,
        Decoration.widget({ widget: new RevertWidget(hot, layer.onRevert), side: 1 }),
      )
    }
  }
  return builder.finish()
}

// firstLines is the usual button placement: each block's own first line.
export function firstLines(blocks: (number | null)[]): Map<number, number> {
  const first = new Map<number, number>()
  for (const [index, block] of blocks.entries()) {
    if (block !== null && !first.has(block)) {
      first.set(block, index + 1)
    }
  }
  return first
}

export function blockHover(): BlockHover {
  const views = new Set<EditorView>()
  let hot: number | null = null

  const light = (next: number | null): void => {
    if (next === hot) {
      return
    }
    hot = next
    for (const view of views) {
      view.dispatch({ effects: setHot.of(next) })
    }
  }

  return {
    layer(options) {
      const tracker = ViewPlugin.define(
        (view) => {
          // A rebuilt editor starts cold, so the next pointer move must be
          // allowed to light it even over the block that was lit before.
          hot = null
          views.add(view)
          return { destroy: () => views.delete(view) }
        },
        {
          eventHandlers: {
            mousemove(event, view) {
              const pos = view.posAtCoords({ x: event.clientX, y: event.clientY })
              const line = pos === null ? null : view.state.doc.lineAt(pos).number
              light(line === null ? null : (options.blocks[line - 1] ?? null))
            },
            mouseleave() {
              light(null)
            },
          },
        },
      )
      return [
        hotField,
        tracker,
        EditorView.decorations.compute([hotField], (state) =>
          buildRevertDecorations(state.doc, options, state.field(hotField)),
        ),
      ]
    },
  }
}
