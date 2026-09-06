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
  ArrowLeft,
  ArrowRight,
  ChevronLeft,
  ChevronRight,
  Clock,
  Code,
  Coins,
  Cpu,
  FileDiff,
  Folder,
  Gauge,
  GitBranch,
  GitPullRequestArrow,
  GripVertical,
  MoreHorizontal,
  Paperclip,
  Timer,
  X,
  type LucideIcon,
} from "lucide-react"
import { dragStyle, useDragSensors } from "@/lib/use-sortable-list"
import {
  FOOTER_ITEMS,
  footerZone,
  hasFooterItem,
  isFooterItem,
  moveFooterItem,
  type FooterItem,
  type FooterLayout,
  type FooterZone,
} from "@/lib/footer-layout"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

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
  available: "Available items",
  left: "Left side",
  right: "Right side",
}
const labelOf = (id: FooterItem) => FOOTER_ITEMS.find((item) => item.id === id)?.label ?? id

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
      <div className="flex min-w-0 flex-col gap-5">
        <FooterZoneView
          zone="available"
          items={available}
          layout={layout}
          disabled={disabled}
          onMove={move}
        />
        <FooterZoneView
          zone="left"
          items={layout.left}
          layout={layout}
          disabled={disabled}
          onMove={move}
        />
        <FooterZoneView
          zone="right"
          items={layout.right}
          layout={layout}
          disabled={disabled}
          onMove={move}
        />
      </div>
      <DragOverlay>
        {dragging && (
          <span className="rounded-md bg-popover px-3 py-2 text-xs shadow-md">
            {labelOf(dragging)}
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
  onMove: (id: FooterItem, target: string) => void
}

function FooterZoneView({ zone, items, layout, disabled, onMove }: FooterZoneViewProps) {
  const { setNodeRef, isOver } = useDroppable({
    id: zone,
    disabled,
    data: { empty: items.length === 0 },
  })
  return (
    <section className="min-w-0" aria-label={LABELS[zone]}>
      <h3 className="mb-2 text-sm font-medium">{LABELS[zone]}</h3>
      <div
        ref={setNodeRef}
        data-footer-zone={zone}
        className={cn(
          "flex min-h-12 flex-wrap items-center gap-1.5 border-y border-border px-1 py-3",
          isOver && "bg-accent/50",
        )}
      >
        <SortableContext items={items} strategy={rectSortingStrategy}>
          {items.map((id) => (
            <FooterEditorItem
              key={id}
              id={id}
              layout={layout}
              disabled={disabled}
              onMove={onMove}
            />
          ))}
        </SortableContext>
        {items.length === 0 && (
          <p className="px-2 py-3 text-xs text-muted-foreground">
            {zone === "available" ? "Drag here to hide an item" : "Drag items here"}
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
  onMove: (id: FooterItem, target: string) => void
}

function FooterEditorItem({ id, layout, disabled, onMove }: FooterEditorItemProps) {
  const {
    setNodeRef,
    setActivatorNodeRef,
    attributes,
    listeners,
    transform,
    transition,
    isDragging,
  } = useSortable({ id, disabled })
  const Icon = ICONS[id]
  return (
    <div
      ref={setNodeRef}
      style={dragStyle(transform, transition)}
      data-footer-item={id}
      className={cn(
        "flex shrink-0 items-center rounded-md bg-accent/30",
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
        className="touch-none cursor-grab active:cursor-grabbing"
      >
        <GripVertical data-icon="inline-start" />
        <Icon />
        {labelOf(id)}
      </Button>
      <FooterItemMenu id={id} layout={layout} disabled={disabled} onMove={onMove} />
      {hasFooterItem(layout, id) && (
        <Button
          variant="ghost"
          size="icon-xs"
          disabled={disabled}
          aria-label={`Remove ${labelOf(id)}`}
          onClick={() => onMove(id, "available")}
        >
          <X />
        </Button>
      )}
    </div>
  )
}

function FooterItemMenu({ id, layout, disabled, onMove }: FooterEditorItemProps) {
  const zone = footerZone(layout, id)
  const items = zone === "available" ? [] : layout[zone]
  const index = items.indexOf(id)
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`Options for ${labelOf(id)}`}
        render={<Button variant="ghost" size="icon-xs" disabled={disabled} />}
      >
        <MoreHorizontal />
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuGroup>
          {zone !== "left" && (
            <DropdownMenuItem onClick={() => onMove(id, "left")}>
              <ArrowLeft />
              {zone === "available" ? "Add to left" : "Move to left"}
            </DropdownMenuItem>
          )}
          {zone !== "right" && (
            <DropdownMenuItem onClick={() => onMove(id, "right")}>
              <ArrowRight />
              {zone === "available" ? "Add to right" : "Move to right"}
            </DropdownMenuItem>
          )}
          {zone !== "available" && (
            <>
              <DropdownMenuItem disabled={index <= 0} onClick={() => onMove(id, items[index - 1])}>
                <ChevronLeft />
                Move earlier
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={index >= items.length - 1}
                onClick={() => onMove(id, items[index + 1])}
              >
                <ChevronRight />
                Move later
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => onMove(id, "available")}>
                <X />
                Remove from footer
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
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
              {layout[side].map((id) => {
                const Icon = ICONS[id]
                return (
                  <span key={id} className="flex items-center gap-1 tabular-nums">
                    <Icon className="size-3" aria-hidden="true" />
                    {FOOTER_ITEMS.find((item) => item.id === id)?.example}
                  </span>
                )
              })}
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
