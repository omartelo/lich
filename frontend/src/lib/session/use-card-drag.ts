import { useState } from "react"
import type { DragEndEvent, DragStartEvent } from "@dnd-kit/core"
import { currentFileDrag, endFileDrag, startFileDrag } from "./file-drag-store"
import type { Session } from "./sessions"

export interface CardDrag {
  /** The card in flight, null between drags. */
  draggingId: string | null
  /** For the block's DndContext. */
  handlers: {
    onDragStart: (event: DragStartEvent) => void
    onDragCancel: () => void
    onDragEnd: (event: DragEndEvent) => void
  }
}

/** One block's card drag. A card let go over a header that takes it is filed
 * there (file-drag-store); anywhere else the drop is `reorder`'s. `onFile` is
 * absent on a block whose cards are never filed by drag, the pinned one and a
 * wall, and there every drop reorders. */
export function useCardDrag(
  sessions: Session[],
  reorder: (event: DragEndEvent) => void,
  onFile?: (sessionId: string, folder: string) => void,
): CardDrag {
  const [draggingId, setDraggingId] = useState<string | null>(null)

  const stop = () => {
    setDraggingId(null)
    endFileDrag()
  }

  return {
    draggingId,
    handlers: {
      onDragStart: ({ active }) => {
        const id = String(active.id)
        setDraggingId(id)
        const session = sessions.find((candidate) => candidate.id === id)
        if (onFile && session) {
          startFileDrag(session.folder ?? "", session.path ?? "")
        }
      },
      onDragCancel: stop,
      onDragEnd: (event) => {
        const into = currentFileDrag()?.over ?? null
        stop()
        if (onFile && into !== null) {
          onFile(String(event.active.id), into)
          return
        }
        reorder(event)
      },
    },
  }
}
