import { NavLink } from "react-router-dom"
import { X } from "lucide-react"
import { useSortable } from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import { cn } from "@/lib/utils"
import type { Project } from "@/lib/api-types"

interface ProjectTabProps {
  project: Project
  onClose: () => void
}

// ProjectTab is a browser-style tab: the project name, active underline, and a
// close affordance that appears on hover. The tab is its own drag grip for
// reordering the strip.
export function ProjectTab({ project, onClose }: ProjectTabProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: project.id })

  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn("shrink-0", isDragging && "z-10 opacity-60")}
      {...attributes}
      {...listeners}
    >
      <NavLink
        to={`/projects/${project.id}`}
        title={project.path}
        className={({ isActive }) =>
          cn(
            "group flex h-8 max-w-52 items-center gap-2 rounded-lg px-3 text-sm text-muted-foreground transition-colors hover:bg-accent/60",
            isActive && "bg-accent text-accent-foreground",
          )
        }
      >
        <span className="truncate">{project.name}</span>
        <span
          role="button"
          aria-label={`Close ${project.name}`}
          onClick={(event) => {
            event.preventDefault()
            onClose()
          }}
          className="flex size-4 shrink-0 items-center justify-center rounded opacity-0 transition-opacity hover:bg-foreground/15 group-hover:opacity-100"
        >
          <X className="size-3" />
        </span>
      </NavLink>
    </div>
  )
}
