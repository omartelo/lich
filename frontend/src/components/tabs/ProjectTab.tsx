import { NavLink } from "react-router-dom"
import { X } from "lucide-react"
import { cn } from "@/lib/utils"
import type { Project } from "../../../bindings/github.com/skipodotdev/skipo/internal/project"

interface ProjectTabProps {
  project: Project
  onClose: () => void
}

// ProjectTab is a browser-style tab: the project name, active underline, and a
// close affordance that appears on hover.
export function ProjectTab({ project, onClose }: ProjectTabProps) {
  return (
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
  )
}
