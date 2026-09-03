import { useEffect, useMemo, useState, useSyncExternalStore } from "react"
import type { KeyboardEvent } from "react"
import { useMatch, useNavigate } from "react-router-dom"
import { DndContext, closestCenter } from "@dnd-kit/core"
import { SortableContext, verticalListSortingStrategy } from "@dnd-kit/sortable"
import { GitPullRequestArrow, PanelLeftClose, Plus, Search } from "lucide-react"
import { toast } from "sonner"
import { ProjectService } from "@/lib/rpc"
import { closeSettings, isSettingsOpen, subscribeSettingsCard } from "@/lib/settings-card-store"
import { closePulls, openPulls } from "@/lib/pulls-card-store"
import { delegateTargets } from "@/lib/session/delegate-targets"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { usePanes } from "@/lib/session/use-panes"
import { useStageToggle } from "@/lib/session/use-stage-toggle"
import {
  closePullsList,
  isPullsListOpen,
  subscribePullsListCard,
} from "@/lib/pulls-list-card-store"
import { enabledProviders, projectDefaultProviderKind, useProviders } from "@/lib/providers-store"
import { Notice } from "@/components/common/Notice"
import { ResizeHandle } from "@/components/common/ResizeHandle"
import { SearchInput } from "@/components/common/SearchInput"
import { SettingsCard } from "./SettingsCard"
import { SidebarCard } from "@/components/common/SidebarCard"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useProjects } from "@/providers/projects"
import { queueSetup } from "@/lib/terminal/setup-queue"
import { filterSessions } from "@/lib/session/session-filter"
import { requestTerminalFocus } from "@/lib/terminal/focus-request"
import {
  activeSessionId,
  dragOrder,
  reorderSubset,
  sessionsOf,
  sidebarGroups,
  type SidebarGroup,
} from "@/lib/session/sessions"
import { useSortableList, verticalAxis, withinList } from "@/lib/use-sortable-list"
import { WorktreeCloseDialogs } from "./WorktreeCloseDialogs"
import { SessionGroup } from "./SessionGroup"
import { WorktreeDialog } from "./WorktreeDialog"
import { useWorktreeClose } from "./useWorktreeClose"
import { useGitStatus } from "@/lib/git/use-git-status"
import { usePanelWidth } from "@/lib/use-panel-width"
import { useWorktreeDialogIntent } from "@/lib/use-sidebar-intent"
import { SessionLaunchMenuItems } from "./SessionLaunchMenuItems"

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
  // The filter over this project's cards. Deliberately neither persisted nor a
  // store: the sidebar's width and collapsed state survive a restart because
  // they are layout, but a filter is a lens held while working — a lich that
  // boots showing two of nine sessions is a bug report, not a restored setting.
  // It drops on a project switch for the reason the dock's file filter does:
  // the list underneath is a different set of sessions entirely.
  const [filterOpen, setFilterOpen] = useState(false)
  const [query, setQuery] = useState("")
  useEffect(() => {
    setFilterOpen(false)
    setQuery("")
  }, [projectId])
  // The shortcut's half of the New session menu's Worktree item. Ungated where
  // the menu item is disabled without a branch: git status may still be loading
  // on the render this mounts in, and the dialog reports git's own error in
  // place anyway.
  useWorktreeDialogIntent(projectId ?? "", () => setWorktreeOpen(true))
  // Resolved ahead of the no-project bail below: hooks cannot sit behind it.
  const list = sessionsOf(sessions, projectId ?? "")
  const worktreeClose = useWorktreeClose(projectId ?? "", path, list)
  const realActiveId = activeSessionId(sessions, projectId ?? "")
  const panes = usePanes(projectId ?? "")
  // The menu entry is a toggle on one card: a session already on the stage takes
  // itself off it, any other joins it. Adding is refused — quietly, the way the
  // shortcut is — when one more pane would leave them all too small to read.
  // Adding a session that is already on another wall takes it off that one —
  // somebody else's arrangement, changed by a click aimed at this one. So the
  // decision goes to the user rather than being made under them; `add` refuses
  // the move until they have answered.
  const { moving, toggleStage, cancelMove, confirmMove } = useStageToggle(panes, list)
  // Gathering an orchestrator and its workers is the one action that builds a
  // whole wall at once. A delegate already on another wall stays there — it is
  // the user's arrangement and taking it is the destructive reading — so the
  // toast names how many were left rather than letting them go missing quietly.
  const groupDelegates = (sessionId: string, delegateIds: string[]) => {
    const skipped = panes.groupWith(sessionId, delegateIds)
    if (skipped > 0) {
      toast(
        skipped === 1
          ? "1 delegate could not be added to the split"
          : `${skipped} delegates could not be added to the split`,
      )
    }
  }

  // The query narrows the flat list before the groups are built from it, so
  // grouping, the pinned block and the stored order all keep working on the
  // survivors without knowing a filter exists.
  const filtering = query.trim() !== ""
  const { sessions: visible, matched } = filterSessions(list, query, path, realActiveId)
  // The split's own block is built from the members, not from what is on screen:
  // a parked wall is exactly the case the user could not see before.
  const groups = sidebarGroups(visible, panes.groups)
  // One drag list for every block that moves — walls and checkouts together, so
  // a wall can be dragged below a worktree instead of being penned in a list of
  // its own. Both kinds sit where their first card sits in the stored flat list,
  // so a drop is one write: move that block's ids inside the list the groups are
  // read back from.
  //
  // The pinned block is the only one out of it: it is always first. Its cards
  // keep their slots in the flat list, so unpinning still drops a card among its
  // old neighbours — which is why the drag only claims the ids its own blocks
  // drew, and not simply every unpinned session (a pinned card on a wall is
  // drawn in the wall, and its slot moves with it).
  const dragKeys = groups.filter((group) => !group.pinned).map((group) => group.key)
  const { sensors, onDragEnd } = useSortableList(dragKeys, (keys) =>
    reorderSessions(projectId ?? "", dragOrder(list, groups, keys)),
  )
  // Resolved once here, not per group: the list spans every open project, so
  // it is the same for every card in the sidebar. Memoised because the picker
  // downstream keys its own flatten and filter off this array's identity, and
  // a fresh one per sidebar render would make those caches never hold.
  const delegateGroups = useMemo(
    () => delegateTargets(projects, sessions, realActiveId),
    [projects, sessions, realActiveId],
  )

  // Closing clears the query: a filter outliving its own field would be hiding
  // cards with nothing on screen to say why. Same reason the collapsed rail
  // never filters — unmounting this sidebar drops the query with it.
  const toggleFilter = () => {
    setQuery("")
    setFilterOpen((open) => !open)
  }

  // Esc with text clears the query and keeps the field; Esc on an empty field
  // closes it and hands the keyboard back to the terminal. Two presses to leave,
  // so a long query is never one keystroke from taking focus off the sidebar.
  const onFilterKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== "Escape") {
      return
    }
    if (query !== "") {
      setQuery("")
      return
    }
    setFilterOpen(false)
    if (realActiveId) {
      requestTerminalFocus(realActiveId)
    }
  }

  if (!projectId) {
    return null
  }

  // No session card highlights while a full-screen route (Settings, Pulls) owns
  // the view; its own sidebar entry reads as active instead.
  const activeId = onSettings || onPullsRoute ? "" : realActiveId
  // Same rule as activeId: with a full-screen route over the terminals there are
  // no panes on show, so no card wears the mark of one.
  const stageIds = onSettings || onPullsRoute ? [] : panes.cells

  // A drag reorders one block only; hand its new order to that block's own
  // sessions inside the flat list and persist the whole thing. reorderSessions
  // bails on any id-set mismatch, so a close that raced the drop drops the
  // stale order.
  const commitGroupOrder = (group: SidebarGroup, ids: string[]) => {
    // A split block is a wall, so a drag in it arranges that wall rather than
    // the stored session list — the same arrangement the drag inside the stage
    // itself makes, from the other end.
    if (group.stage) {
      panes.reorderCells(group.stage.id, ids)
      return
    }
    // The slots the drag may fill are the cards this block drew, asked of the
    // block itself rather than restated as a rule about pins and paths: a
    // checkout whose sibling sits on a wall has that card in neither, and a
    // predicate that claimed it anyway would hand reorderSubset one id too many
    // — an id-set mismatch, which reorderSessions drops whole.
    const drawn = new Set(group.sessions.map((session) => session.id))
    reorderSessions(
      projectId,
      reorderSubset(list, ids, (session) => drawn.has(session.id)),
    )
  }

  const createWorktree = async (
    name: string,
    base: string,
    baseIsRemote: boolean,
    sandbox: string,
  ) => {
    const wt = await ProjectService.CreateWorktree(path, projectId, name, base, baseIsRemote)
    if (wt) {
      // A fresh checkout is the one moment the project's setup script runs;
      // reopening an existing worktree never queues it.
      queueSetup(newWorktreeSession(projectId, wt, sandbox))
    }
    setWorktreeOpen(false)
  }

  const resumeWorktree = (wt: { name: string; path: string }) => {
    void reopenWorktreeSession(projectId, wt)
    setWorktreeOpen(false)
  }

  // One block, rendered the same whichever drag list it belongs to.
  const renderGroup = (group: SidebarGroup) => {
    const groupActive = group.sessions.some((s) => s.id === realActiveId)
    return (
      <SessionGroup
        // The project is in the React key, not only in the props: two
        // projects both have a root block and a pinned one under the
        // same key, so without it React reuses one project's group
        // component for the other's — carrying its fold across with it,
        // and never re-reading the fold the switched-to project stored.
        key={`${projectId}:${group.key}`}
        sortId={group.key}
        pinned={group.pinned}
        stage={group.stage}
        onRenameGroup={(name) => group.stage && panes.rename(group.stage.id, name)}
        onDissolveGroup={() => group.stage && panes.dissolve(group.stage.id)}
        projectId={projectId}
        path={group.path}
        sessions={group.sessions}
        projectPath={path}
        activeId={activeId}
        stageIds={stageIds}
        onStageToggle={toggleStage}
        onGroupDelegates={groupDelegates}
        // The divider only earns its place once a worktree — or a pin
        // — splits the list; a lone group keeps the old flat,
        // header-less look. A filter is the exception: which checkout
        // a surviving card sits in is the thing the query was typed to
        // find out, so the title stays even for a lone group.
        showHeader={groups.length > 1 || filtering}
        // A drop computed from a filtered view hands reorderSubset an
        // id set that does not name the group's members, and
        // reorderSessions rejects the whole order — so the gesture
        // would be a silent no-op. Take it away instead of leaving a
        // dead one on screen.
        sortable={!filtering}
        onReorder={(ids) => commitGroupOrder(group, ids)}
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
        delegateGroups={delegateGroups}
        providers={enabled}
      />
    )
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
            <SessionLaunchMenuItems
              providers={enabled}
              terminalLabel="Terminal"
              onNewSession={(kind) => newSession(projectId, kind)}
              worktree={{
                disabled: !git?.branch,
                onSelect: () => setWorktreeOpen(true),
              }}
            />
          </DropdownMenuContent>
        </DropdownMenu>
        <Button
          variant="ghost"
          title="Filter sessions"
          aria-label="Filter sessions"
          aria-pressed={filterOpen}
          onClick={toggleFilter}
          className="size-8 shrink-0 justify-center px-0 text-muted-foreground hover:bg-accent hover:text-foreground aria-pressed:bg-accent aria-pressed:text-foreground"
        >
          <Search className="size-4" />
        </Button>
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
      {/* Revealed rather than resident: 36px is half a session card off a list
          that already scrolls, and unlike the dock's file tree — a panel opened
          to find something — the sidebar is watched all day and opened for
          nothing. */}
      {filterOpen && (
        <div className="mb-2">
          <SearchInput
            // The field exists only once the magnifier is pressed, and that press
            // is the request for it.
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={onFilterKeyDown}
            placeholder="Filter sessions"
            aria-label="Filter sessions"
            spellCheck={false}
            className="h-7 text-xs"
          />
        </div>
      )}
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
        {/* Not `groups.length === 0`: the active card is kept whatever the
            query says, so the list is rarely empty and the sentence has to
            explain the one card that is still there. */}
        {filtering && !matched && (
          <Notice className="px-2 py-3">
            No sessions match “{query.trim()}”. The active session stays.
          </Notice>
        )}
        {/* Every block in one list, in the order sidebarGroups drew them: the
            pinned one first and fixed, the rest — walls and checkouts alike —
            sortable among each other. */}
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          modifiers={[verticalAxis, withinList]}
          onDragEnd={onDragEnd}
        >
          <SortableContext items={dragKeys} strategy={verticalListSortingStrategy}>
            {groups.map(renderGroup)}
          </SortableContext>
        </DndContext>
      </div>

      <WorktreeDialog
        open={worktreeOpen}
        onOpenChange={setWorktreeOpen}
        projectPath={path}
        projectId={projectId}
        providerId={projectDefaultProviderKind(projectId)}
        currentBranch={git?.branch ?? ""}
        onCreate={createWorktree}
        onResume={resumeWorktree}
      />
      <WorktreeCloseDialogs close={worktreeClose} />
      <ConfirmDialog
        open={!!moving}
        onCancel={cancelMove}
        // "this split" only when there is one: adding to no wall starts a new
        // one around the active session, and naming a split the user cannot see
        // is the same lie as the entry that promised to stop showing a card.
        title={
          panes.current
            ? `Move ${moving?.session.label ?? ""} to this split?`
            : `Show ${moving?.session.label ?? ""} beside this session?`
        }
        description={
          moving?.from.cells.length === 2 ? (
            <>
              It is one of only two sessions in <strong>{moving.from.name}</strong>, so that group
              ends when it leaves. No session is closed either way.
            </>
          ) : (
            <>
              It is on <strong>{moving?.from.name}</strong>, and a session can only be on one split
              at a time — it leaves that one. No session is closed either way.
            </>
          )
        }
      >
        <Button onClick={confirmMove}>Move it</Button>
      </ConfirmDialog>

      <ResizeHandle edge="right" label="Resize sidebar" handleProps={handleProps} />
    </aside>
  )
}
