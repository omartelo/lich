// The card being dragged, as the block headers see it: whether their block
// can take it, and whether the pointer is on them right now.
//
// A card's drag lives in its own block's DndContext, and no droppable of another
// block can register with it, so the headers of the other blocks learn about the
// drag here instead, and the drag finds them by hit-testing the DOM for the
// attribute a header wears only while it takes the card (FILE_TARGET_ATTRIBUTE).
// The hit test follows the real pointer: the coordinates dnd-kit hands its
// handlers are the card's, after the modifiers and the auto-scroll moved it.

import { type CollisionDetection, closestCenter, type Modifier } from "@dnd-kit/core"

/** The card in flight: the folder it is filed under ("" for none), the checkout
 * it belongs to ("" for the project root), and the folder the header under the
 * pointer would file it into. `over` is null off every target, and "" on its own
 * checkout's header, which takes it out of its folder. */
export interface FileDrag {
  folder: string
  path: string
  over: string | null
}

/** What a block header files a dropped card into: a folder by name, or "" for
 * a checkout's header, whose `path` names the checkout. */
export interface DropTarget {
  folder: string
  path: string
}

/** What a header draws during a drag: nothing ("idle": no drag), dimmed
 * ("refuses"), a target ("accepts"), or the target under the pointer ("over"). */
export type DropState = "idle" | "refuses" | "accepts" | "over"

export const FILE_TARGET_ATTRIBUTE = "data-file-target"

// How far under the pointer a card hangs while a header is under it: enough to
// clear the header's own row.
const HANG_BELOW_POINTER_PX = 8

let current: FileDrag | null = null
const listeners = new Set<() => void>()

function emit(): void {
  for (const listener of listeners) {
    listener()
  }
}

function track(event: PointerEvent): void {
  setFileDropOver(fileTargetAt(event.clientX, event.clientY))
}

/** Starts following the pointer for targets until endFileDrag. A keyboard drag
 * moves no pointer, so it never reaches a header: the card menu is that path. */
export function startFileDrag(folder: string, path: string): void {
  current = { folder, path, over: null }
  window.addEventListener("pointermove", track)
  emit()
}

/** Only a change notifies: the pointer moves every frame, and a header repaints
 * when the target under it changes, not when the pointer does. */
export function setFileDropOver(over: string | null): void {
  if (!current || current.over === over) {
    return
  }
  current = { ...current, over }
  emit()
}

export function endFileDrag(): void {
  if (!current) {
    return
  }
  window.removeEventListener("pointermove", track)
  current = null
  emit()
}

export function currentFileDrag(): FileDrag | null {
  return current
}

export function subscribeFileDrag(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

// A folder takes any card not already in it. A checkout's header takes only a
// filed card of its own checkout: filing never moves a session between
// checkouts, so the header of another one would promise a move that cannot
// happen.
function accepts(target: DropTarget, drag: FileDrag): boolean {
  if (target.folder) {
    return target.folder !== drag.folder
  }
  return drag.folder !== "" && target.path === drag.path
}

/** How a header with `target` (null for a block nothing can be filed into: the
 * pinned one, a wall) answers the drag in flight. */
export function dropState(target: DropTarget | null, drag: FileDrag | null): DropState {
  if (!drag) {
    return "idle"
  }
  if (!target || !accepts(target, drag)) {
    return "refuses"
  }
  return drag.over === target.folder ? "over" : "accepts"
}

/** The folder the header under a viewport point files into, or null when no
 * target is there. The dragged card is pointer-events-none, so it never hides
 * the header beneath it from the hit test. */
export function fileTargetAt(x: number, y: number): string | null {
  const target = document.elementFromPoint(x, y)?.closest(`[${FILE_TARGET_ATTRIBUTE}]`)
  return target?.getAttribute(FILE_TARGET_ATTRIBUTE) ?? null
}

/** closestCenter, except over a header that takes the card, where the card is
 * over itself: the list makes no room for a drop that will file it instead. Not
 * over nothing, which dnd-kit answers by dropping the card back in its slot
 * while the pointer is still on the header. */
export const fileAwareCollision: CollisionDetection = (args) =>
  (currentFileDrag()?.over ?? null) === null ? closestCenter(args) : [{ id: args.active.id }]

/** Hangs the card just under the pointer while a header takes it. Carried by its
 * middle, the card would cover the very header it is about to be filed by. */
export const hangBelowTarget: Modifier = ({ activatorEvent, draggingNodeRect, transform }) => {
  const over = currentFileDrag()?.over ?? null
  if (over === null || !draggingNodeRect || !(activatorEvent instanceof MouseEvent)) {
    return transform
  }
  const pointerY = activatorEvent.clientY + transform.y
  return { ...transform, y: pointerY + HANG_BELOW_POINTER_PX - draggingNodeRect.top }
}
