import { useSyncExternalStore } from "react"
import type { PaneGroup } from "./panes"
import type { Session } from "./sessions"

// The session waiting on the user's answer before it is taken off another wall.
// A module store rather than one component's state: the sidebar's card, the
// palette and a pane's + all ask the same question, and the dialog that asks it
// is mounted once beside the stage, where every one of them can reach it.

export interface StageMove {
  session: Session
  /** The wall it would leave. */
  from: PaneGroup
}

let pending: StageMove | null = null
const listeners = new Set<() => void>()

function emit(): void {
  for (const listener of listeners) {
    listener()
  }
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function requestStageMove(move: StageMove): void {
  pending = move
  emit()
}

export function clearStageMove(): void {
  pending = null
  emit()
}

export function useStageMove(): StageMove | null {
  return useSyncExternalStore(subscribe, () => pending)
}
