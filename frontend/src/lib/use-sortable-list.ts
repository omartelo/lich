import {
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core"
import { arrayMove, sortableKeyboardCoordinates } from "@dnd-kit/sortable"

// How far the pointer must travel before a press turns into a drag. Without it
// the sensor claims the press outright and a plain click stops selecting the
// session or navigating to the tab.
const DRAG_THRESHOLD_PX = 5

// useSortableList wires the sensors and the drop handler shared by the two
// reorderable surfaces (session cards, project tabs). It reports the new id
// order once, when a drag actually lands somewhere new.
//
// dnd-kit rides pointer events, so the drag never reaches the native GTK/X11
// drag-and-drop the webview would otherwise hand it to — and it animates with
// CSS transforms rather than reordering the DOM, so no rearranged-list state has
// to exist while a drag is in flight.
export function useSortableList(ids: string[], onCommit: (ids: string[]) => void) {
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: DRAG_THRESHOLD_PX },
    }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const onDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id) {
      return
    }
    const from = ids.indexOf(String(active.id))
    const to = ids.indexOf(String(over.id))
    if (from === -1 || to === -1) {
      return
    }
    onCommit(arrayMove(ids, from, to))
  }

  return { sensors, onDragEnd }
}
