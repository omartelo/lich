import { Link } from "react-router-dom"
import { Bell, Check, LoaderCircle } from "lucide-react"
import { useSortable } from "@dnd-kit/sortable"
import { CloseButton } from "@/components/common/CloseButton"
import { dragStyle } from "@/lib/use-sortable-list"
import { cn } from "@/lib/utils"
import { useProjectStatus } from "@/lib/session/use-session-status"
import type { Project } from "@/lib/api-types"

interface ProjectTabProps {
  project: Project
  sessionIds: readonly string[]
  // Where the tab leads: the screen this project was last showing, which is not
  // always its terminals (project-route).
  to: string
  active: boolean
  onClose: () => void
}

// The tab is its own drag grip for reordering the strip — no separate handle.
export function ProjectTab({ project, sessionIds, to, active, onClose }: ProjectTabProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: project.id,
  })
  // What the project's sessions are up to while you are looking elsewhere. The
  // active tab never badges: its cards are already on screen saying the same
  // thing, in more detail and per session.
  const status = useProjectStatus(sessionIds)
  const badge = active ? null : status

  return (
    <div
      ref={setNodeRef}
      style={dragStyle(transform, transition)}
      className={cn("shrink-0", isDragging && "z-10 rounded-md bg-accent shadow-md")}
      {...attributes}
      {...listeners}
    >
      <Link
        to={to}
        title={project.path}
        className={cn(
          "group flex h-8 max-w-52 items-center gap-2 rounded-md px-3 text-sm text-muted-foreground transition-colors hover:text-foreground",
          active && "bg-accent font-medium text-accent-foreground",
        )}
      >
        {badge === "busy" && <LoaderCircle className="size-3 shrink-0 animate-spin" />}
        {badge === "done" && <Check className="size-3 shrink-0 text-emerald-500" />}
        {badge === "waiting" && <Bell className="size-3 shrink-0 text-amber-500" />}
        <span className="truncate">{project.name}</span>
        {/* preventDefault, not stopPropagation: the parent is a link, and the
            click must not navigate to the tab being closed. */}
        <CloseButton
          label={`Close ${project.name}`}
          onClick={(event) => {
            event.preventDefault()
            onClose()
          }}
        />
      </Link>
    </div>
  )
}
