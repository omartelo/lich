import { useRef, useState } from "react"
import type { PointerEvent as ReactPointerEvent } from "react"
import { useMatch } from "react-router-dom"
import { Plus, X } from "lucide-react"
import { cn } from "@/lib/utils"
import { useProjects } from "@/lib/projects"
import { activeSessionId, sessionsOf, type Session } from "@/lib/sessions"

// Sidebar width bounds in rem, matching the Tailwind v4 spacing scale. State and
// storage stay in rem; the pointer drag delta arrives in CSS pixels and is
// converted with the 16px root font size Tailwind assumes.
const REM_PX = 16
const MIN_WIDTH_REM = 12
const MAX_WIDTH_REM = 30
const DEFAULT_WIDTH_REM = 15
const WIDTH_STORAGE_KEY = "skipo.sidebar.width"

const clampWidth = (rem: number): number =>
  Math.min(MAX_WIDTH_REM, Math.max(MIN_WIDTH_REM, rem))

function readWidth(): number {
  const stored = Number(localStorage.getItem(WIDTH_STORAGE_KEY))
  return Number.isFinite(stored) && stored > 0
    ? clampWidth(stored)
    : DEFAULT_WIDTH_REM
}

// SessionRow is one session entry: the label plus a close button on hover.
//
// ponytail: this is where git branch/diff and inline rename will hang off a
// session once those services exist — the row already reserves the room.
function SessionRow({
  session,
  active,
  onSelect,
  onClose,
}: {
  session: Session
  active: boolean
  onSelect: () => void
  onClose: () => void
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      title={session.label}
      className={cn(
        "group flex h-9 w-full items-center gap-2 rounded-lg px-3 text-sm text-muted-foreground transition-colors hover:bg-accent/60",
        active && "bg-accent text-accent-foreground",
      )}
    >
      <span className="flex-1 truncate text-left">{session.label}</span>
      <span
        role="button"
        aria-label={`Close ${session.label}`}
        onClick={(event) => {
          event.stopPropagation()
          onClose()
        }}
        className="flex size-4 shrink-0 items-center justify-center rounded opacity-0 transition-opacity hover:bg-foreground/15 group-hover:opacity-100"
      >
        <X className="size-3" />
      </span>
    </button>
  )
}

// SessionSidebar lists the active project's sessions and can be drag-resized
// within a fixed pixel range. Width persists across restarts. It renders nothing
// when no project is active (Home, Settings), so it never competes with those
// screens.
//
// Resizing only changes this element's width; the terminal keeps its PTY in sync
// on its own via a ResizeObserver, so the sidebar does not need to know about it.
export function SessionSidebar() {
  const { sessions, newSession, closeSession, activateSession } = useProjects()
  const match = useMatch("/projects/:projectId")
  const projectId = match?.params.projectId
  const [width, setWidth] = useState(readWidth)
  const dragRef = useRef<{ startX: number; startWidth: number } | null>(null)

  if (!projectId) {
    return null
  }

  const list = sessionsOf(sessions, projectId)
  const activeId = activeSessionId(sessions, projectId)

  const startDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault()
    dragRef.current = { startX: event.clientX, startWidth: width }
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  const onDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current
    if (!drag) {
      return
    }
    setWidth(clampWidth(drag.startWidth + (event.clientX - drag.startX) / REM_PX))
  }

  const endDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current
    if (!drag) {
      return
    }
    const finalWidth = clampWidth(
      drag.startWidth + (event.clientX - drag.startX) / REM_PX,
    )
    dragRef.current = null
    event.currentTarget.releasePointerCapture(event.pointerId)
    setWidth(finalWidth)
    localStorage.setItem(WIDTH_STORAGE_KEY, String(finalWidth))
  }

  return (
    <aside
      className="relative flex shrink-0 flex-col border-r border-border bg-sidebar p-2"
      style={{ width: `${width}rem` }}
    >
      <div className="flex flex-1 flex-col gap-1 overflow-y-auto">
        {list.map((session) => (
          <SessionRow
            key={session.id}
            session={session}
            active={session.id === activeId}
            onSelect={() => activateSession(projectId, session.id)}
            onClose={() => closeSession(projectId, session.id)}
          />
        ))}
        <button
          type="button"
          onClick={() => newSession(projectId)}
          title="New session"
          aria-label="New session"
          className="flex h-9 w-full items-center gap-2 rounded-lg px-3 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          <Plus className="size-4 shrink-0" />
          <span>New session</span>
        </button>
      </div>

      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize sidebar"
        onPointerDown={startDrag}
        onPointerMove={onDrag}
        onPointerUp={endDrag}
        className="absolute right-0 top-0 h-full w-1.5 cursor-col-resize touch-none transition-colors hover:bg-accent"
      />
    </aside>
  )
}
