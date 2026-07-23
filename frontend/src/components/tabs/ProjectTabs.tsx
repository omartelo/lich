import {useMatch, useNavigate} from "react-router-dom"
import {Plus, Settings} from "lucide-react"
import {DndContext, closestCenter} from "@dnd-kit/core"
import {SortableContext, horizontalListSortingStrategy} from "@dnd-kit/sortable"
import {cn} from "@/lib/utils"
import {Button} from "@/components/ui/button"
import {useProjects} from "@/lib/projects"
import {sessionsOf} from "@/lib/sessions"
import {openSettings} from "@/lib/settings-card-store"
import {NotificationsButton} from "./NotificationsButton"
import {useSortableList} from "@/lib/use-sortable-list"
import {ProjectTab} from "./ProjectTab"
import {HomeTab} from "./HomeTab"

export function ProjectTabs() {
  const {projects, sessions, homeId, openProject, closeProject, reorderProjects} =
    useProjects()
  const navigate = useNavigate()
  // The project the settings gear targets: whichever one is in view, falling
  // back to Home when the app is on the bare landing screen.
  const activeProjectId = useMatch("/projects/:projectId/*")?.params.projectId ?? homeId
  const onSettings = !!useMatch("/projects/:projectId/settings")

  const openProjectSettings = () => {
    if (!activeProjectId) {
      return
    }
    openSettings(activeProjectId)
    navigate(`/projects/${activeProjectId}/settings`)
  }
  // Home is pinned first and stays out of the drag list so it never reorders.
  const rest = projects.filter((project) => project.id !== homeId)
  const ids = rest.map((project) => project.id)
  const {sensors, onDragEnd} = useSortableList(ids, reorderProjects)
  const showHome = homeId !== null && projects.some((p) => p.id === homeId)

  return (
    <div className="flex h-11 shrink-0 items-center gap-1 border-b border-border bg-sidebar px-2">
      <div className="flex flex-1 items-center gap-1 overflow-x-auto">
        {showHome && homeId && <HomeTab projectId={homeId}/>}
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
          <Plus className="size-4"/>
        </Button>
      </div>
      <NotificationsButton/>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={openProjectSettings}
        disabled={!activeProjectId}
        title="Settings"
        aria-label="Settings"
        className={cn(
          "shrink-0 text-muted-foreground",
          onSettings && "bg-accent text-accent-foreground",
        )}
      >
        <Settings className="size-4"/>
      </Button>
    </div>
  )
}
