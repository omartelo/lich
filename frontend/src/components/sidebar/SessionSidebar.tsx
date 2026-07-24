import {useState, useSyncExternalStore} from "react"
import {useMatch, useNavigate} from "react-router-dom"
import {GitBranch, Plus, Terminal} from "lucide-react"
import {toast} from "sonner"
import {ProjectService, Store} from "@/lib/rpc"
import {closeSettings, isSettingsOpen, subscribeSettingsCard} from "@/lib/settings-card-store"
import {enabledProviders, useProviders} from "@/lib/providers-store"
import {ProviderIcon} from "@/lib/provider-icons"
import {SettingsCard} from "./SettingsCard"
import {Button} from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {useProjects} from "@/lib/projects"
import {queueSetup} from "@/lib/setup-queue"
import {
  activeSessionId,
  groupByWorktree,
  isLastWorktreeSession,
  sessionsOf,
  type Session,
} from "@/lib/sessions"
import {baseName} from "@/lib/paths"
import {CloseWorktreeDialog, ForceRemoveWorktreeDialog} from "./CloseWorktreeDialog"
import {SessionGroup} from "./SessionGroup"
import {WorktreeDialog} from "./WorktreeDialog"
import {useGitStatus} from "@/lib/useGitStatus"
import {usePanelWidth} from "@/lib/use-panel-width"
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
    reopenWorktreeSession,
    closeSession,
    keepSession,
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
  const enabled = enabledProviders(useProviders())
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

  if (!projectId) {
    return null
  }

  // No card is highlighted while the Settings screen owns the view, so the
  // Settings card reads as the active one instead.
  const activeId = onSettings ? "" : activeSessionId(sessions, projectId)
  const groups = groupByWorktree(list)
  const projectName = baseName(path)

  // A drag reorders one group only; splice its new order back into the flat list
  // in group order and persist the whole thing. reorderSessions bails on any
  // id-set mismatch, so a close that raced the drop drops the stale order.
  const commitGroupOrder = (groupPath: string, ids: string[]) => {
    const flat = groups.flatMap((group) =>
      group.path === groupPath ? ids : group.sessions.map((session) => session.id),
    )
    reorderSessions(projectId, flat)
  }

  const createWorktree = async (name: string, base: string, baseIsRemote: boolean) => {
    const wt = await ProjectService.CreateWorktree(path, projectId, name, base, baseIsRemote)
    if (wt) {
      // A fresh checkout is the one moment the project's setup script runs;
      // reopening an existing worktree never queues it.
      queueSetup(newWorktreeSession(projectId, wt))
    }
    setWorktreeOpen(false)
  }

  const resumeWorktree = (wt: { name: string; path: string }) => {
    void reopenWorktreeSession(projectId, wt)
    setWorktreeOpen(false)
  }

  const requestClose = (session: Session) => {
    if (isLastWorktreeSession(list, session)) {
      setPendingClose(session)
      return
    }
    closeSession(projectId, session.id)
  }

  const keepAndClose = () => {
    if (pendingClose) {
      keepSession(projectId, pendingClose.id)
    }
    setPendingClose(null)
  }

  // Close first so the PTY running inside the worktree dies before git tries
  // to remove it. A refused removal surfaces as a toast; the checkout stays on
  // disk and reappears in the new-worktree picker.
  const closeAndRemove = (session: Session, force: boolean) => {
    closeSession(projectId, session.id)
    // The checkout is going away, so no parked row for it may linger — one would
    // otherwise resurface a resume against a worktree that no longer exists.
    void Store.PurgeWorktreeSessions(projectId, session.path ?? "")
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
      <DropdownMenu>
        <DropdownMenuTrigger
          title="New session"
          aria-label="New session"
          render={
            <Button
              variant="ghost"
              className="mb-2 w-full justify-start gap-2 text-foreground hover:bg-accent aria-expanded:bg-accent"
            />
          }
        >
          <Plus className="size-4 text-muted-foreground"/>
          New Session
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="max-w-56">
          <DropdownMenuGroup>
            {enabled.map((provider) => (
              <DropdownMenuItem
                key={provider.id}
                onClick={() => newSession(projectId, provider.id)}
              >
                <ProviderIcon kind={provider.id}/>
                {provider.name}
              </DropdownMenuItem>
            ))}
          </DropdownMenuGroup>
          <DropdownMenuSeparator/>
          <DropdownMenuGroup>
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
      <div className="flex flex-1 flex-col gap-1.5 overflow-y-auto overflow-x-hidden">
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
        {groups.map((group) => (
          <SessionGroup
            key={group.path || "__root__"}
            path={group.path}
            sessions={group.sessions}
            projectPath={path}
            projectName={projectName}
            activeId={activeId}
            // The divider only earns its place once a worktree splits the list;
            // a lone group keeps the old flat, header-less look.
            showHeader={groups.length > 1}
            onReorder={(ids) => commitGroupOrder(group.path, ids)}
            onSelect={(id) => {
              activateSession(projectId, id)
              // From the settings screen this returns to the terminal; on the
              // project route it is a no-op.
              navigate(`/projects/${projectId}`)
            }}
            onClose={(session) => requestClose(session)}
            onRename={(id, label) => renameSession(projectId, id, label)}
            onOpenTerminal={(cwd) => newSession(projectId, "shell", cwd)}
          />
        ))}
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
