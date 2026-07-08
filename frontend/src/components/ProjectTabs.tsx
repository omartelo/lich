import { NavLink } from "react-router-dom"
import { Plus, Settings, X } from "lucide-react"
import { cn } from "@/lib/utils"
import { useProjects } from "@/lib/projects"
import type { Project } from "../../bindings/github.com/skipodotdev/skipo/internals/project"

// ProjectTab is a browser-style tab: the project name, active underline, and a
// close affordance that appears on hover.
function ProjectTab({
  project,
  onClose,
}: {
  project: Project
  onClose: () => void
}) {
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

// ProjectTabs is the top strip: open projects as tabs, a button to open another,
// and settings pinned to the right.
export function ProjectTabs() {
  const { projects, openProject, closeProject } = useProjects()

  return (
    <div className="flex h-11 shrink-0 items-center gap-1 border-b border-border bg-sidebar px-2">
      <div className="flex flex-1 items-center gap-1 overflow-x-auto">
        {projects.map((project) => (
          <ProjectTab
            key={project.id}
            project={project}
            onClose={() => closeProject(project.id)}
          />
        ))}
        <button
          type="button"
          onClick={() => void openProject()}
          title="Open project"
          aria-label="Open project"
          className="flex size-8 shrink-0 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          <Plus className="size-4" />
        </button>
      </div>

      <NavLink
        to="/settings"
        title="Settings"
        aria-label="Settings"
        className={({ isActive }) =>
          cn(
            "flex size-8 shrink-0 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
            isActive && "bg-accent text-accent-foreground",
          )
        }
      >
        <Settings className="size-4" />
      </NavLink>
    </div>
  )
}
