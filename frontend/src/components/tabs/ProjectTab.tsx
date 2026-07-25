import { NavLink } from "react-router-dom"
import { Bell, Check, LoaderCircle } from "lucide-react"
import { useSortable } from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import { CloseButton } from "@/components/common/CloseButton"
import { cn } from "@/lib/utils"
import { useProjectStatus } from "@/lib/useSessionStatus"
import type { Project } from "@/lib/api-types"

interface ProjectTabProps {
  project: Project
  sessionIds: readonly string[]
  onClose: () => void
}

// The tab is its own drag grip for reordering the strip — no separate handle.
export function ProjectTab({ project, sessionIds, onClose }: ProjectTabProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: project.id,
  })
  // What the project's sessions are up to while you are looking elsewhere. The
  // active tab never badges: its cards are already on screen saying the same
  // thing, in more detail and per session.
  const status = useProjectStatus(sessionIds)

  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn("shrink-0", isDragging && "z-10 rounded-md bg-accent shadow-md")}
      {...attributes}
      {...listeners}
    >
      <NavLink
        to={`/projects/${project.id}`}
        title={project.path}
        className={({ isActive }) =>
          cn(
            "group flex h-8 max-w-52 items-center gap-2 rounded-md px-3 text-sm text-muted-foreground transition-colors hover:text-foreground",
            isActive && "bg-accent font-medium text-accent-foreground",
          )
        }
      >
        {({ isActive }) => {
          const badge = isActive ? null : status
          return (
            <>
              {badge === "busy" && <LoaderCircle className="size-3 shrink-0 animate-spin" />}
              {badge === "done" && <Check className="size-3 shrink-0 text-emerald-500" />}
              {badge === "waiting" && <Bell className="size-3 shrink-0 text-amber-500" />}
              <span className="truncate">{project.name}</span>
              {/* preventDefault, not stopPropagation: the parent is a NavLink,
                  and the click must not navigate to the tab being closed. */}
              <CloseButton
                label={`Close ${project.name}`}
                onClick={(event) => {
                  event.preventDefault()
                  onClose()
                }}
              />
            </>
          )
        }}
      </NavLink>
    </div>
  )
}
