import {useState, useSyncExternalStore} from "react"
import {useMatch, useNavigate} from "react-router-dom"
import {Bot, GitBranch, Plus, Terminal} from "lucide-react"
import {toast} from "sonner"
import {ProjectService} from "@/lib/rpc"
import {closeSettings, isSettingsOpen, subscribeSettingsCard} from "@/lib/settings-card-store"
import {SettingsCard} from "./SettingsCard"
import {Button} from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {useProjects} from "@/lib/projects"
import {activeSessionId, sessionsOf, type Session} from "@/lib/sessions"
import {CloseWorktreeDialog, ForceRemoveWorktreeDialog} from "./CloseWorktreeDialog"
import {SessionCard} from "./SessionCard"
import {WorktreeDialog} from "./WorktreeDialog"
import {DndContext, closestCenter} from "@dnd-kit/core"
import {SortableContext, verticalListSortingStrategy} from "@dnd-kit/sortable"
import {useGitStatus} from "@/lib/useGitStatus"
import {usePanelWidth} from "@/lib/use-panel-width"
import {useSortableList} from "@/lib/use-sortable-list"
import {errorText} from "@/lib/utils"

// SessionSidebar lists the active project's sessions and can be drag-resized
// within a fixed pixel range. Width persists across restarts. It renders nothing
// when no project is active (Home, Settings), so it never competes with those
// screens.
//
// Resizing only changes this element's width; the terminal keeps its PTY in sync
// on its own via a ResizeObserver, so the sidebar does not need to know about it.
export function SessionSidebar() {
  const {
    projects,
    sessions,
    newSession,
    newWorktreeSession,
    closeSession,
    activateSession,
    renameSession,
    reorderSessions,
  } = useProjects()
  // Match the project subtree ("/*") so the sidebar stays mounted — and keeps
  // resolving its project — while the per-project Settings screen is open.
  const match = useMatch("/projects/:projectId/*")
  const projectId = match?.params.projectId
  const onSettings = !!useMatch("/projects/:projectId/settings")
  const navigate = useNavigate()
  const settingsOpen = useSyncExternalStore(subscribeSettingsCard, () =>
    isSettingsOpen(projectId ?? ""),
  )
  const path = projects.find((p) => p.id === projectId)?.path ?? ""
  const git = useGitStatus(path)
  const {width, handleProps} = usePanelWidth({
    storageKey: "lich.sidebar.width",
    minRem: 12,
    maxRem: 30,
    defaultRem: 15,
    edge: "right",
  })
  const [worktreeOpen, setWorktreeOpen] = useState(false)
  const [pendingClose, setPendingClose] = useState<Session | null>(null)
  const [pendingForce, setPendingForce] = useState<Session | null>(null)
  // Resolved ahead of the no-project bail below: hooks cannot sit behind it.
  const list = sessionsOf(sessions, projectId ?? "")
  const {sensors, onDragEnd} = useSortableList(
    list.map((session) => session.id),
    (ids) => reorderSessions(projectId ?? "", ids),
  )

  if (!projectId) {
    return null
  }

  const activeId = activeSessionId(sessions, projectId)

  const createWorktree = async (name: string, base: string, baseIsRemote: boolean) => {
    const wt = await ProjectService.CreateWorktree(path, projectId, name, base, baseIsRemote)
    if (wt) {
      newWorktreeSession(projectId, wt)
    }
    setWorktreeOpen(false)
  }

  const resumeWorktree = (wt: { name: string; path: string }) => {
    newWorktreeSession(projectId, wt)
    setWorktreeOpen(false)
  }

  // Closing a worktree session asks what to do with the checkout; regular
  // sessions close immediately.
  const requestClose = (session: Session) => {
    if (session.path) {
      setPendingClose(session)
      return
    }
    closeSession(projectId, session.id)
  }

  const keepAndClose = () => {
    if (pendingClose) {
      closeSession(projectId, pendingClose.id)
    }
    setPendingClose(null)
  }

  // Close first so the PTY running inside the worktree dies before git tries
  // to remove it. A refused removal surfaces as a toast; the checkout stays on
  // disk and reappears in the new-worktree picker.
  const closeAndRemove = (session: Session, force: boolean) => {
    closeSession(projectId, session.id)
    ProjectService.RemoveWorktree(path, session.path ?? "", force).catch(
      (err: unknown) => {
        toast.error(`Failed to remove worktree: ${errorText(err)}`)
      },
    )
  }

  const removeAndClose = async () => {
    const session = pendingClose
    setPendingClose(null)
    if (!session?.path) {
      return
    }
    // A dirty worktree needs a second confirmation before --force discards its
    // changes. A failed check falls through to the plain remove, whose own
    // refusal surfaces as a toast.
    const dirty = await ProjectService.WorktreeDirty(session.path).catch(
      () => false,
    )
    if (dirty) {
      setPendingForce(session)
      return
    }
    closeAndRemove(session, false)
  }

  const forceRemoveAndClose = () => {
    const session = pendingForce
    setPendingForce(null)
    if (session?.path) {
      closeAndRemove(session, true)
    }
  }

  return (
    <aside
      className="relative flex shrink-0 flex-col border-r border-border bg-sidebar p-2"
      style={{width: `${width}rem`}}
    >
      <div className="mb-2 flex items-center justify-between px-1">
        <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Sessions
        </span>
        <DropdownMenu>
          <DropdownMenuTrigger
            title="New session"
            aria-label="New session"
            render={<Button variant="ghost" size="icon-xs" className="text-muted-foreground"/>}
          >
            <Plus className="size-4"/>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className={"w-44"}>
            <DropdownMenuGroup>
              <DropdownMenuItem onClick={() => newSession(projectId, "claude")}>
                <Bot/>
                Claude Code
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => newSession(projectId, "shell")}>
                <Terminal/>
                Terminal
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={!git?.branch}
                onClick={() => setWorktreeOpen(true)}
              >
                <GitBranch/>
                Worktree
              </DropdownMenuItem>
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <div className="flex flex-1 flex-col gap-1.5 overflow-y-auto">
        {settingsOpen && (
          <SettingsCard
            active={onSettings}
            onSelect={() => navigate(`/projects/${projectId}/settings`)}
            onClose={() => {
              closeSettings(projectId)
              // Leaving settings drops back to the project's active terminal.
              if (onSettings) {
                navigate(`/projects/${projectId}`)
              }
            }}
          />
        )}
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={onDragEnd}
        >
          <SortableContext
            items={list.map((session) => session.id)}
            strategy={verticalListSortingStrategy}
          >
            {list.map((session) => (
              <SessionCard
                key={session.id}
                session={session}
                path={path}
                // A session is never highlighted while the Settings screen owns
                // the view, so the Settings card reads as the active one.
                active={session.id === activeId && !onSettings}
                onSelect={() => {
                  activateSession(projectId, session.id)
                  // From the settings screen this returns to the terminal; on
                  // the project route it is a no-op.
                  navigate(`/projects/${projectId}`)
                }}
                onClose={() => requestClose(session)}
                onRename={(label) => renameSession(projectId, session.id, label)}
              />
            ))}
          </SortableContext>
        </DndContext>
      </div>

      <WorktreeDialog
        open={worktreeOpen}
        onOpenChange={setWorktreeOpen}
        projectPath={path}
        currentBranch={git?.branch ?? ""}
        onCreate={createWorktree}
        onResume={resumeWorktree}
      />
      <CloseWorktreeDialog
        session={pendingClose}
        onCancel={() => setPendingClose(null)}
        onKeep={keepAndClose}
        onRemove={removeAndClose}
      />
      <ForceRemoveWorktreeDialog
        session={pendingForce}
        onCancel={() => setPendingForce(null)}
        onForceRemove={forceRemoveAndClose}
      />

      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize sidebar"
        {...handleProps}
        className="absolute right-0 top-0 h-full w-1.5 cursor-col-resize touch-none transition-colors hover:bg-accent"
      />
    </aside>
  )
}
