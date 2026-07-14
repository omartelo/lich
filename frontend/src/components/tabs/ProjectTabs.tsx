import { NavLink } from "react-router-dom"
import { Plus, Settings } from "lucide-react"
import { DndContext, closestCenter } from "@dnd-kit/core"
import { SortableContext, horizontalListSortingStrategy } from "@dnd-kit/sortable"
import { cn } from "@/lib/utils"
import { useProjects } from "@/lib/projects"
import { useSortableList } from "@/lib/use-sortable-list"
import { ProjectTab } from "./ProjectTab"

// ProjectTabs is the top strip: open projects as tabs (drag to reorder), a
// button to open another, and settings pinned to the right.
export function ProjectTabs() {
  const { projects, openProject, closeProject, reorderProjects } = useProjects()
  const ids = projects.map((project) => project.id)
  const { sensors, onDragEnd } = useSortableList(ids, reorderProjects)

  return (
    <div className="flex h-11 shrink-0 items-center gap-1 border-b border-border bg-sidebar px-2">
      <div className="flex flex-1 items-center gap-1 overflow-x-auto">
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={onDragEnd}
        >
          <SortableContext items={ids} strategy={horizontalListSortingStrategy}>
            {projects.map((project) => (
              <ProjectTab
                key={project.id}
                project={project}
                onClose={() => closeProject(project.id)}
              />
            ))}
          </SortableContext>
        </DndContext>
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
