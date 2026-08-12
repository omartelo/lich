import { useState, useSyncExternalStore } from "react"
import { useMatch, useNavigate } from "react-router-dom"
import { DndContext, closestCenter } from "@dnd-kit/core"
import { SortableContext, verticalListSortingStrategy } from "@dnd-kit/sortable"
import { GitBranch, GitPullRequestArrow, PanelLeftClose, Plus, Terminal } from "lucide-react"
import { ProjectService } from "@/lib/rpc"
import { closeSettings, isSettingsOpen, subscribeSettingsCard } from "@/lib/settings-card-store"
import { closePulls, openPulls } from "@/lib/pulls-card-store"
import { mentionTargets } from "@/lib/session/mention-targets"
import { delegateTargets } from "@/lib/session/delegate-targets"
import {
  closePullsList,
  isPullsListOpen,
  subscribePullsListCard,
} from "@/lib/pulls-list-card-store"
import { enabledProviders, useProviders } from "@/lib/providers-store"
import { ProviderIcon } from "@/components/ProviderIcon"
import { ResizeHandle } from "@/components/common/ResizeHandle"
import { SettingsCard } from "./SettingsCard"
import { SidebarCard } from "@/components/common/SidebarCard"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useProjects } from "@/providers/projects"
import { queueSetup } from "@/lib/terminal/setup-queue"
import {
  activeSessionId,
  groupByWorktree,
  groupKey,
  orderGroups,
  sessionsOf,
  sortPinned,
} from "@/lib/session/sessions"
import { useSortableList, verticalAxis } from "@/lib/use-sortable-list"
import { CloseWorktreeDialog, ForceRemoveWorktreeDialog } from "./CloseWorktreeDialog"
import { SessionGroup } from "./SessionGroup"
import { WorktreeDialog } from "./WorktreeDialog"
import { useWorktreeClose } from "./useWorktreeClose"
import { useGitStatus } from "@/lib/git/use-git-status"
import { usePanelWidth } from "@/lib/use-panel-width"

// Named here like every other `lich.*` pref rather than spelled at the call
// site. The bounds go with it: wide enough for a session label and its branch,
// capped short of crowding the terminal.
const WIDTH_KEY = "lich.sidebar.width"
const MIN_REM = 12
const MAX_REM = 30
const DEFAULT_REM = 15

interface SessionSidebarProps {
  // Collapses the sidebar to its rail (SidebarRail), which carries the way back.
  onCollapse: () => void
}

// SessionSidebar lists the active project's sessions and can be drag-resized
// within a fixed pixel range. Width persists across restarts. It renders nothing
// when no project is active (Home, Settings), so it never competes with those
// screens.
//
// Resizing only changes this element's width; the terminal keeps its PTY in sync
// on its own via a ResizeObserver, so the sidebar does not need to know about it.
export function SessionSidebar({ onCollapse }: SessionSidebarProps) {
  const {
    projects,
    sessions,
    newSession,
    newWorktreeSession,
    reopenWorktreeSession,
    activateSession,
    reorderSessions,
  } = useProjects()
  // Match the project subtree ("/*") so the sidebar stays mounted — and keeps
  // resolving its project — while the per-project Settings screen is open.
  const match = useMatch("/projects/:projectId/*")
  const projectId = match?.params.projectId
  const onSettings = !!useMatch("/projects/:projectId/settings")
  // Two screens, two cards: a worktree's own pull request (exact route) and the
  // repository's list (the "all" subtree, whose selection moves the URL).
  const onPullsRoute = !!useMatch("/projects/:projectId/pulls")
  const onPullsListRoute = !!useMatch("/projects/:projectId/pulls/all/*")
  const navigate = useNavigate()
  const settingsOpen = useSyncExternalStore(subscribeSettingsCard, () =>
    isSettingsOpen(projectId ?? ""),
  )
  const pullsListOpen = useSyncExternalStore(subscribePullsListCard, () =>
    isPullsListOpen(projectId ?? ""),
  )
  const enabled = enabledProviders(useProviders())
  const path = projects.find((p) => p.id === projectId)?.path ?? ""
  const git = useGitStatus(path)
  const { width, handleProps } = usePanelWidth({
    storageKey: WIDTH_KEY,
    minRem: MIN_REM,
    maxRem: MAX_REM,
    defaultRem: DEFAULT_REM,
    edge: "right",
  })
  const [worktreeOpen, setWorktreeOpen] = useState(false)
  // Resolved ahead of the no-project bail below: hooks cannot sit behind it.
  // Pinned first, which also carries their worktree group to the top of the
  // sidebar — groupByWorktree buckets by first appearance.
  const list = sortPinned(sessionsOf(sessions, projectId ?? ""))
  const worktreeClose = useWorktreeClose(projectId ?? "", path, list)
  const groups = groupByWorktree(list)
  // Dragging a group moves its whole block of sessions inside the flat list the
  // groups are read back from — there is no separate group order to store.
  const { sensors, onDragEnd } = useSortableList(
    groups.map((group) => groupKey(group.path)),
    (keys) => reorderSessions(projectId ?? "", orderGroups(groups, keys)),
  )

  if (!projectId) {
    return null
  }

  const realActiveId = activeSessionId(sessions, projectId)
  // Resolved once here, not per group: the list spans every open project, so
  // it is the same for every card in the sidebar.
  const mentionGroups = mentionTargets(projects, sessions, realActiveId)
  const delegateGroups = delegateTargets(projects, sessions, realActiveId)
  // No session card highlights while a full-screen route (Settings, Pulls) owns
  // the view; its own sidebar entry reads as active instead.
  const activeId = onSettings || onPullsRoute ? "" : realActiveId

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

  return (
    <aside
      className="relative flex shrink-0 flex-col border-r border-border bg-sidebar p-2"
      style={{ width: `${width}rem` }}
    >
      <div className="mb-2 flex items-center gap-1">
        <DropdownMenu>
          <DropdownMenuTrigger
            title="New session"
            aria-label="New session"
            render={
              <Button
                variant="ghost"
                className="w-full flex-1 justify-start gap-2 text-foreground hover:bg-accent aria-expanded:bg-accent"
              />
            }
          >
            <Plus className="size-4 text-muted-foreground" />
            New Session
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="max-w-56">
            <DropdownMenuGroup>
              {enabled.map((provider) => (
                <DropdownMenuItem
                  key={provider.id}
                  onClick={() => newSession(projectId, provider.id)}
                >
                  <ProviderIcon kind={provider.id} />
                  {provider.name}
                </DropdownMenuItem>
              ))}
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuItem onClick={() => newSession(projectId, "shell")}>
                <Terminal />
                Terminal
              </DropdownMenuItem>
              <DropdownMenuItem disabled={!git?.branch} onClick={() => setWorktreeOpen(true)}>
                <GitBranch />
                Worktree
              </DropdownMenuItem>
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button
          variant="ghost"
          title="Collapse sidebar"
          aria-label="Collapse sidebar"
          onClick={onCollapse}
          className="size-8 shrink-0 justify-center px-0 text-muted-foreground hover:bg-accent hover:text-foreground"
        >
          <PanelLeftClose className="size-4" />
        </Button>
      </div>
      {/* The scrollbar takes width, so it rides the aside's own padding (-mr-2)
          and the padding is re-applied inside — a gap between thumb and card
          when it shows, no shift in card width when it doesn't. */}
      <div className="-mr-2 flex flex-1 flex-col gap-1.5 overflow-y-auto overflow-x-hidden pr-2">
        {/* Pinned above the session groups beside Settings, not inside a
            worktree's group: this card belongs to the project, and the screen it
            opens is the repository's, not one checkout's. */}
        {pullsListOpen && (
          <SidebarCard
            icon={GitPullRequestArrow}
            label="Pull requests"
            active={onPullsListRoute}
            onSelect={() => navigate(`/projects/${projectId}/pulls/all`)}
            onClose={() => {
              closePullsList(projectId)
              if (onPullsListRoute) {
                navigate(`/projects/${projectId}`)
              }
            }}
            closeLabel="Close pull requests"
          />
        )}
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
          modifiers={[verticalAxis]}
          onDragEnd={onDragEnd}
        >
          <SortableContext
            items={groups.map((group) => groupKey(group.path))}
            strategy={verticalListSortingStrategy}
          >
            {groups.map((group) => {
              const groupActive = group.sessions.some((s) => s.id === realActiveId)
              return (
                <SessionGroup
                  key={groupKey(group.path)}
                  projectId={projectId}
                  path={group.path}
                  sessions={group.sessions}
                  projectPath={path}
                  activeId={activeId}
                  // The divider only earns its place once a worktree splits the
                  // list; a lone group keeps the old flat, header-less look.
                  showHeader={groups.length > 1}
                  onReorder={(ids) => commitGroupOrder(group.path, ids)}
                  onClose={worktreeClose.requestClose}
                  pullsActive={onPullsRoute && groupActive}
                  onPulls={() => {
                    openPulls(group.path || path)
                    const target = groupActive ? realActiveId : group.sessions[0]?.id
                    if (target) {
                      activateSession(projectId, target)
                    }
                    navigate(`/projects/${projectId}/pulls`)
                  }}
                  onClosePulls={() => {
                    closePulls(group.path || path)
                    if (onPullsRoute && groupActive) {
                      navigate(`/projects/${projectId}`)
                    }
                  }}
                  mentionGroups={mentionGroups}
                  delegateGroups={delegateGroups}
                />
              )
            })}
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
        session={worktreeClose.pendingClose}
        onCancel={worktreeClose.cancel}
        onKeep={worktreeClose.keep}
        onRemove={worktreeClose.remove}
      />
      <ForceRemoveWorktreeDialog
        session={worktreeClose.pendingForce}
        onCancel={worktreeClose.cancel}
        onForceRemove={worktreeClose.forceRemove}
      />

      <ResizeHandle edge="right" label="Resize sidebar" handleProps={handleProps} />
    </aside>
  )
}
