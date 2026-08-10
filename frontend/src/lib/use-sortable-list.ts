import {
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type Modifier,
} from "@dnd-kit/core"
import { arrayMove, sortableKeyboardCoordinates } from "@dnd-kit/sortable"
import { CSS, type Transform } from "@dnd-kit/utilities"

// How far the pointer must travel before a press turns into a drag. Without it
// the sensor claims the press outright and a plain click stops selecting the
// session or navigating to the tab.
const DRAG_THRESHOLD_PX = 5

export const horizontalAxis: Modifier = ({ transform }) => ({ ...transform, y: 0 })
export const verticalAxis: Modifier = ({ transform }) => ({ ...transform, x: 0 })

// The style every sortable node wears while it moves. Translate, never
// Transform: dnd-kit's transform also carries the scale needed to match the
// item being displaced, so a card dragged past a taller neighbour would stretch
// to that neighbour's size instead of keeping its own.
export function dragStyle(transform: Transform | null, transition?: string) {
  return { transform: CSS.Translate.toString(transform), transition }
}

// useSortableList wires the sensors and the drop handler shared by the two
// reorderable surfaces (session cards, project tabs). It reports the new id
// order once, when a drag actually lands somewhere new.
//
// dnd-kit rides pointer events instead of the HTML5 DnD API — and it animates
// with CSS transforms rather than reordering the DOM, so no rearranged-list
// state has to exist while a drag is in flight.
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
