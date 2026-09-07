import { useState } from "react"
import {
  DndContext,
  DragOverlay,
  closestCenter,
  pointerWithin,
  rectIntersection,
  useDroppable,
  type CollisionDetection,
  type DragEndEvent,
  type KeyboardCoordinateGetter,
} from "@dnd-kit/core"
import {
  SortableContext,
  rectSortingStrategy,
  sortableKeyboardCoordinates,
  useSortable,
} from "@dnd-kit/sortable"
import {
  Clock,
  Code,
  Coins,
  Cpu,
  FileDiff,
  Folder,
  Gauge,
  GitBranch,
  GitPullRequestArrow,
  Paperclip,
  Timer,
  type LucideIcon,
} from "lucide-react"
import { dragStyle, useDragSensors } from "@/lib/use-sortable-list"
import { formatModel } from "@/lib/model-name"
import { useActiveSession } from "@/lib/session/use-active-session"
import { useSessionAgent } from "@/lib/session/use-session-agent"
import { useSessionUsage } from "@/lib/session/use-session-usage"
import { ProviderIcon } from "@/components/ProviderIcon"
import {
  FOOTER_ITEMS,
  hasFooterItem,
  isFooterItem,
  moveFooterItem,
  type FooterItem,
  type FooterLayout,
  type FooterZone,
} from "@/lib/footer-layout"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

const ICONS: Record<FooterItem, LucideIcon> = {
  attach: Paperclip,
  files: Code,
  changes: FileDiff,
  pr: GitPullRequestArrow,
  checkout: GitBranch,
  path: Folder,
  model: Cpu,
  context: Gauge,
  plan: Gauge,
  cost: Coins,
  handsOn: Timer,
  clock: Clock,
}
const LABELS: Record<FooterZone, string> = {
  available: "Available",
  left: "Left",
  right: "Right",
}
const exampleOf = (id: FooterItem) => FOOTER_ITEMS.find((item) => item.id === id)?.example ?? id
const labelOf = (id: FooterItem) => FOOTER_ITEMS.find((item) => item.id === id)?.label ?? id

interface ItemReadingProps {
  id: FooterItem
  /** The preview bar sets its own smaller glyph. */
  small?: boolean
}

// What the item puts in the footer: its glyph and its reading. One component
// for the chip, the drag overlay and the preview bar, so the thing under the
// cursor is the thing that was picked up.
function ItemReading({ id, small }: ItemReadingProps) {
  if (id === "model") {
    return <ModelReading small={small} />
  }
  const Icon = ICONS[id]
  return (
    <>
      <Icon className={small ? "size-3" : undefined} aria-hidden="true" />
      {exampleOf(id)}
    </>
  )
}

// The model slot is the one item whose footer reading is not a shape but a
// fact: the provider's own mark and the model the active session is running,
// which is what SessionModel draws. A made-up "Codex · GPT-6" named a provider
// the footer never writes and pinned a model nobody chose.
function ModelReading({ small }: { small?: boolean }) {
  const { sessionId, kind } = useActiveSession()
  const usage = useSessionUsage(sessionId)
  const provider = useSessionAgent(sessionId) ?? kind
  if (!usage?.model || !provider) {
    const Icon = ICONS.model
    return (
      <>
        <Icon className={small ? "size-3" : undefined} aria-hidden="true" />
        {exampleOf("model")}
      </>
    )
  }
  return (
    <>
      <ProviderIcon kind={provider} size={small ? 12 : 14} />
      {formatModel(usage.model)}
      {usage.effort ? ` · ${usage.effort}` : ""}
    </>
  )
}

// Only a pointer inside a drop area may commit; inside it, items take precedence.
const collisionDetection: CollisionDetection = (args) => {
  const hits = args.pointerCoordinates ? pointerWithin(args) : rectIntersection(args)
  if (
    args.pointerCoordinates &&
    !hits.some((hit) => ["available", "left", "right"].includes(String(hit.id)))
  )
    return []
  const items = hits.filter((hit) => hit.id !== args.active.id && isFooterItem(String(hit.id)))
  return items.length ? items : hits.length || args.pointerCoordinates ? hits : closestCenter(args)
}

// A populated zone's rectangle would steal arrow-key moves from its items.
// Keep only empty zone rectangles; they are the targets with no item to aim at.
const keyboardCoordinates: KeyboardCoordinateGetter = (event, args) => {
  const droppableRects = new Map(args.context.droppableRects)
  const current = args.context.collisionRect
  const horizontal = event.code === "ArrowLeft" || event.code === "ArrowRight"
  for (const [id, rect] of droppableRects) {
    if (horizontal && current && (rect.bottom <= current.top || rect.top >= current.bottom))
      droppableRects.delete(id)
  }
  for (const zone of ["available", "left", "right"]) {
    if (!args.context.droppableContainers.get(zone)?.data.current?.empty)
      droppableRects.delete(zone)
  }
  return sortableKeyboardCoordinates(event, {
    ...args,
    context: { ...args.context, droppableRects },
  })
}

interface FooterLayoutEditorProps {
  layout: FooterLayout
  disabled: boolean
  onChange: (layout: FooterLayout) => void
}

export function FooterLayoutEditor({ layout, disabled, onChange }: FooterLayoutEditorProps) {
  const sensors = useDragSensors(keyboardCoordinates)
  const [dragging, setDragging] = useState<FooterItem | null>(null)
  const move = (id: FooterItem, target: string) => {
    const next = moveFooterItem(layout, id, target)
    if (next !== layout) onChange(next)
  }
  const drop = ({ active, over }: DragEndEvent) => {
    setDragging(null)
    if (!disabled && over && isFooterItem(String(active.id)))
      move(String(active.id) as FooterItem, String(over.id))
  }
  const available = FOOTER_ITEMS.map((item) => item.id).filter((id) => !hasFooterItem(layout, id))
  return (
    <DndContext
      sensors={sensors}
      collisionDetection={collisionDetection}
      onDragStart={({ active }) =>
        isFooterItem(String(active.id)) && setDragging(String(active.id) as FooterItem)
      }
      onDragEnd={drop}
      onDragCancel={() => setDragging(null)}
    >
      <div className="flex min-w-0 flex-col gap-4">
        {/* Two columns, one per end of the real footer, so an item sits on the
            side it will show on. Available lands below them: what is in the
            footer is what the pane is about. */}
        <div className="grid min-w-0 grid-cols-2 gap-3">
          <FooterZoneView zone="left" items={layout.left} layout={layout} disabled={disabled} />
          <FooterZoneView zone="right" items={layout.right} layout={layout} disabled={disabled} />
        </div>
        <FooterZoneView zone="available" items={available} layout={layout} disabled={disabled} />
      </div>
      <DragOverlay>
        {dragging && (
          <span className="flex items-center gap-1.5 rounded-md bg-popover px-2.5 py-1.5 text-xs shadow-md">
            <ItemReading id={dragging} />
          </span>
        )}
      </DragOverlay>
    </DndContext>
  )
}

interface FooterZoneViewProps {
  zone: FooterZone
  items: FooterItem[]
  layout: FooterLayout
  disabled: boolean
}

function FooterZoneView({ zone, items, layout, disabled }: FooterZoneViewProps) {
  const { setNodeRef, isOver } = useDroppable({
    id: zone,
    disabled,
    data: { empty: items.length === 0 },
  })
  const side = zone !== "available"
  return (
    <section className="flex min-w-0 flex-col" aria-label={LABELS[zone]}>
      <h3 className="mb-1.5 text-xs text-muted-foreground">{LABELS[zone]}</h3>
      <div
        ref={setNodeRef}
        data-footer-zone={zone}
        className={cn(
          "flex flex-1 flex-wrap content-start items-center gap-1.5 rounded-lg p-2",
          side ? "min-h-18 bg-sidebar" : "min-h-11",
          // The outline is the answer to "does it land here", so it only exists
          // while something is in the air; a permanent one reads as a border.
          isOver && "ring-2 ring-ring",
          zone === "right" && "justify-end",
        )}
      >
        <SortableContext items={items} strategy={rectSortingStrategy}>
          {items.map((id) => (
            <FooterEditorItem key={id} id={id} layout={layout} disabled={disabled} />
          ))}
        </SortableContext>
        {items.length === 0 && (
          <p className="px-1 text-xs text-muted-foreground">
            {zone === "available" ? "Every item is in the footer" : "Drag items here"}
          </p>
        )}
      </div>
    </section>
  )
}

interface FooterEditorItemProps {
  id: FooterItem
  layout: FooterLayout
  disabled: boolean
}

function FooterEditorItem({ id, layout, disabled }: FooterEditorItemProps) {
  const {
    setNodeRef,
    setActivatorNodeRef,
    attributes,
    listeners,
    transform,
    transition,
    isDragging,
  } = useSortable({ id, disabled })
  const inFooter = hasFooterItem(layout, id)
  return (
    <div
      ref={setNodeRef}
      style={dragStyle(transform, transition)}
      data-footer-item={id}
      className={cn(
        // The chip carries the reading the footer will show, not the setting's
        // name: an item is recognised by what it puts on screen. Nothing on it
        // appears on hover: a control that grows under the pointer moves the
        // chips beside it, which is exactly the moment a drag is being aimed.
        "flex shrink-0 items-center rounded-md ring-1 ring-inset ring-border",
        inFooter ? "bg-background" : "bg-transparent text-muted-foreground",
        isDragging && "opacity-30",
      )}
    >
      <Button
        ref={setActivatorNodeRef}
        variant="ghost"
        size="xs"
        disabled={disabled}
        {...attributes}
        {...listeners}
        aria-label={`Move ${labelOf(id)}`}
        className="touch-none cursor-grab tabular-nums active:cursor-grabbing"
      >
        <ItemReading id={id} />
      </Button>
    </div>
  )
}

interface FooterLayoutPreviewProps {
  layout: FooterLayout
}

export function FooterLayoutPreview({ layout }: FooterLayoutPreviewProps) {
  return (
    <section className="mt-5 flex flex-col gap-2" aria-label="Footer preview">
      <span className="text-xs text-muted-foreground">
        Preview · example values · scroll to see all items
      </span>
      <section
        className="overflow-x-auto bg-sidebar"
        // biome-ignore lint/a11y/noNoninteractiveTabindex: scrollable region needs keyboard access
        tabIndex={0}
        aria-label="Scrollable footer preview"
      >
        <div className="flex min-h-12 w-max min-w-full items-center justify-between gap-8 px-3 py-2">
          {(["left", "right"] as const).map((side) => (
            <div
              key={side}
              data-preview-side={side}
              className={cn(
                "flex shrink-0 items-center gap-3 whitespace-nowrap text-xs text-muted-foreground",
                side === "right" && "ml-auto justify-end",
              )}
            >
              {layout[side].map((id) => (
                <span key={id} className="flex items-center gap-1 tabular-nums">
                  <ItemReading id={id} small />
                </span>
              ))}
            </div>
          ))}
          {layout.left.length + layout.right.length === 0 && (
            <span className="text-xs text-muted-foreground">No footer items selected</span>
          )}
        </div>
      </section>
    </section>
  )
}
