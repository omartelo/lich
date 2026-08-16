import { useState } from "react"
import { useMatch, useNavigate } from "react-router-dom"
import { GitPullRequestArrow, Settings } from "lucide-react"
import { DndContext, closestCenter } from "@dnd-kit/core"
import { SortableContext, horizontalListSortingStrategy } from "@dnd-kit/sortable"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { useProjects } from "@/providers/projects"
import { sessionsOf } from "@/lib/session/sessions"
import { runningSessions } from "@/lib/session/use-session-status"
import type { Project } from "@/lib/api-types"
import { openSettings } from "@/lib/settings-card-store"
import { openPullsList } from "@/lib/pulls-list-card-store"
import { useSettings } from "@/providers/settings"
import { useHotkey } from "@/lib/use-hotkey"
import { NotificationsButton } from "./NotificationsButton"
import { horizontalAxis, useSortableList, withinList } from "@/lib/use-sortable-list"
import { ProjectTab } from "./ProjectTab"
import { HomeTab } from "./HomeTab"
import { OpenProjectMenu } from "./OpenProjectMenu"

export function ProjectTabs() {
  const { projects, sessions, homeId, closeProject, reorderProjects } = useProjects()
  const navigate = useNavigate()
  // The project waiting on the "sessions are still running" confirmation, with
  // how many were running when it was asked for. Closing a project unmounts its
  // terminals, which kills their PTYs — an agent mid-turn dies with them, and no
  // undo brings that turn back, so this one asks first.
  const [pending, setPending] = useState<{ project: Project; running: number } | null>(null)
  // The project the settings gear targets: whichever one is in view, falling
  // back to Home when the app is on the bare landing screen.
  const activeProjectId = useMatch("/projects/:projectId/*")?.params.projectId ?? homeId
  const onSettings = !!useMatch("/projects/:projectId/settings")
  const onPulls = !!useMatch("/projects/:projectId/pulls/all/*")

  const openProjectSettings = () => {
    if (!activeProjectId) {
      return
    }
    openSettings(activeProjectId)
    navigate(`/projects/${activeProjectId}/settings`)
  }

  // The repository's pull requests, and the only way in that does not already
  // require one: every other entry — the session card's badge, the footer's,
  // the worktree's parked card — appears once that checkout has a pull request,
  // which is exactly when a list of the others is not what is missing. Those
  // stay what they were, a single pull request on its own; this one is the list.
  const openProjectPulls = () => {
    if (!activeProjectId) {
      return
    }
    openPullsList(activeProjectId)
    navigate(`/projects/${activeProjectId}/pulls/all`)
  }
  // Both shortcuts open exactly what their toolbar button opens, and decline the
  // press when that button would be disabled — with no project there is nothing
  // to configure and no repository to read. The pull request screen speaks for
  // itself when the project has no repository or no open pull request.
  const { hotkeys } = useSettings()
  useHotkey(hotkeys.settings, () => (activeProjectId ? openProjectSettings() : false))
  useHotkey(hotkeys.pulls, () => (activeProjectId ? openProjectPulls() : false))

  const requestClose = (project: Project) => {
    const running = runningSessions(sessionsOf(sessions, project.id).map((s) => s.id))
    if (running.length === 0) {
      closeProject(project.id)
      return
    }
    setPending({ project, running: running.length })
  }

  // Home is pinned first and stays out of the drag list so it never reorders.
  const rest = projects.filter((project) => project.id !== homeId)
  const ids = rest.map((project) => project.id)
  const { sensors, onDragEnd } = useSortableList(ids, reorderProjects)
  const showHome = homeId !== null && projects.some((p) => p.id === homeId)

  return (
    <>
      <div className="flex h-10 shrink-0 items-center gap-1 border-b border-border bg-sidebar px-2">
        <div className="flex flex-1 items-center gap-1 overflow-x-auto overflow-y-hidden">
          {showHome && homeId && <HomeTab projectId={homeId} />}
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            modifiers={[horizontalAxis, withinList]}
            onDragEnd={onDragEnd}
          >
            <SortableContext items={ids} strategy={horizontalListSortingStrategy}>
              {rest.map((project) => (
                <ProjectTab
                  key={project.id}
                  project={project}
                  sessionIds={sessionsOf(sessions, project.id).map((s) => s.id)}
                  onClose={() => requestClose(project)}
                />
              ))}
            </SortableContext>
          </DndContext>
          <OpenProjectMenu />
        </div>
        <div aria-hidden className="mx-1 h-5 w-px shrink-0 bg-border" />
        <NotificationsButton />
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={openProjectPulls}
          disabled={!activeProjectId}
          title="Pull requests"
          aria-label="Pull requests"
          className={cn(
            "shrink-0 text-muted-foreground",
            onPulls && "bg-accent text-accent-foreground",
          )}
        >
          <GitPullRequestArrow className="size-4" />
        </Button>
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
          <Settings className="size-4" />
        </Button>
      </div>
      <ConfirmDialog
        open={pending !== null}
        onCancel={() => setPending(null)}
        title="Sessions are still running"
        description={
          <>
            Closing <span className="font-medium">{pending?.project.name}</span> stops{" "}
            {pending?.running === 1 ? "a session" : `${pending?.running} sessions`} mid-turn.
            Reopening the project offers to resume where each left off; the turn in flight is lost.
          </>
        }
      >
        <Button
          variant="destructive"
          onClick={() => {
            if (pending) {
              closeProject(pending.project.id)
            }
            setPending(null)
          }}
        >
          Close anyway
        </Button>
      </ConfirmDialog>
    </>
  )
}
