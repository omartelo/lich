import { NavLink } from "react-router-dom"
import { Plus, Settings } from "lucide-react"
import { DndContext, closestCenter } from "@dnd-kit/core"
import { SortableContext, horizontalListSortingStrategy } from "@dnd-kit/sortable"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { useProjects } from "@/lib/projects"
import { sessionsOf } from "@/lib/sessions"
import { useSortableList } from "@/lib/use-sortable-list"
import { ProjectTab } from "./ProjectTab"
import { HomeTab } from "./HomeTab"

// ProjectTabs is the top strip: the pinned Home tab, open projects as tabs
// (drag to reorder), a button to open another, and settings pinned to the right.
export function ProjectTabs() {
  const { projects, sessions, homeId, openProject, closeProject, reorderProjects } =
    useProjects()
  // Home is pinned first and stays out of the drag list so it never reorders.
  const rest = projects.filter((project) => project.id !== homeId)
  const ids = rest.map((project) => project.id)
  const { sensors, onDragEnd } = useSortableList(ids, reorderProjects)
  const showHome = homeId !== null && projects.some((p) => p.id === homeId)

  return (
    <div className="flex h-11 shrink-0 items-center gap-1 border-b border-border bg-sidebar px-2">
      <div className="flex flex-1 items-center gap-1 overflow-x-auto">
        {showHome && homeId && <HomeTab projectId={homeId} />}
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={onDragEnd}
        >
          <SortableContext items={ids} strategy={horizontalListSortingStrategy}>
            {rest.map((project) => (
              <ProjectTab
                key={project.id}
                project={project}
                sessionIds={sessionsOf(sessions, project.id).map((s) => s.id)}
                onClose={() => closeProject(project.id)}
              />
            ))}
          </SortableContext>
        </DndContext>
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => void openProject()}
          title="Open project"
          aria-label="Open project"
          className="text-muted-foreground"
        >
          <Plus className="size-4" />
        </Button>
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
