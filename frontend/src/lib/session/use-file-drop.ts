import { useSyncExternalStore } from "react"
import {
  currentFileDrag,
  type DropState,
  type DropTarget,
  dropState,
  subscribeFileDrag,
} from "./file-drag-store"

/** How this block's header answers the card being dragged. A string snapshot,
 * so a header re-renders when its own answer changes and not on every change
 * of the drag. */
export function useFileDrop(target: DropTarget | null): DropState {
  return useSyncExternalStore(subscribeFileDrag, () => dropState(target, currentFileDrag()))
}
