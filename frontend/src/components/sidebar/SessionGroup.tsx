import { useState, useSyncExternalStore } from "react"
import { useNavigate } from "react-router-dom"
import { DndContext, closestCenter } from "@dnd-kit/core"
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable"
import { dragStyle, useSortableList, verticalAxis, withinList } from "@/lib/use-sortable-list"
import { cn } from "@/lib/utils"
import { checkoutLabel } from "@/lib/git/checkout-label"
import type { ProviderState } from "@/lib/providers-store"
import type { DelegateGroup } from "@/lib/session/delegate-targets"
import { type Session, sessionOrigin } from "@/lib/session/sessions"
import { useProjects } from "@/providers/projects"
import { SessionCard } from "./SessionCard"
import { PullRequestCard } from "./PullRequestCard"
import { isPullsOpen, subscribePullsCard } from "@/lib/pulls-card-store"
import { SessionGroupHeader } from "./SessionGroupHeader"

interface SessionGroupProps {
  projectId: string
  // This block's sortable id — the checkout path, or a stand-in for the two
  // blocks that have none (the project root, the pinned sessions).
  sortId: string
  // The block of pinned sessions, gathered from every checkout: titled after
  // the pin rather than a worktree, no pull request of its own, and never
  // dragged — it is always the first block.
  pinned: boolean
  // "" for the project's own root or the pinned block, else the worktree
  // checkout path.
  path: string
  sessions: Session[]
  projectPath: string
  activeId: string
  // A divider label is drawn only when the sidebar holds more than one group; a
  // lone project with no worktrees keeps its old flat, header-less list. The
  // header doubles as the group's drag handle, so a lone group is also the case
  // where reordering groups has nothing to reorder.
  showHeader: boolean
  // Commits a new order for this group's sessions; a drag can only ever produce
  // one, since each group owns an isolated DndContext. Not derivable here: the
  // sidebar splices it back into the flat list its siblings share.
  onReorder: (ids: string[]) => void
  // Closing stays the sidebar's: the last session of a worktree raises the
  // keep-or-remove dialog it owns (useWorktreeClose).
  onClose: (session: Session) => void
  // The worktree's pull-request entry: opens the Pulls screen for this branch.
  // pullsActive marks it when that screen is showing this group's PR. Rendered
  // only for worktree groups (a truthy path).
  pullsActive: boolean
  onPulls: () => void
  onClosePulls: () => void
  // Workspace-wide, so it is resolved once by the sidebar rather than per group:
  // the sessions the active one can hand work to, across every open project.
  delegateGroups: DelegateGroup[]
  // The globally enabled providers, already resolved by the sidebar for its
  // own New Session menu. A group offers the same roster in its checkout.
  providers: ProviderState[]
}

// SessionGroup renders one worktree's sessions under a static divider titled
// with the worktree's name; the branch stays on each card. Both the title and
// the bucketing read the group's own checkout path, never a session's live cwd,
// so a `cd` deeper into the tree never re-buckets the group or renames it. The
// isolated DndContext is what confines a card drag to reordering within the
// group; the group itself is a sortable of the sidebar's outer context, dragged
// by its header alone so the two never contend for the same pointer.
//
// The card actions needing nothing but a session id — select, rename, open a
// terminal beside it — are wired here rather than threaded down from the
// sidebar, which keeps only the ones carrying its own state.
export function SessionGroup({
  projectId,
  sortId,
  pinned,
  path,
  sessions,
  projectPath,
  activeId,
  showHeader,
  onReorder,
  onClose,
  pullsActive,
  onPulls,
  onClosePulls,
  delegateGroups,
  providers,
}: SessionGroupProps) {
  const {
    sessions: workspace,
    activateSession,
    renameSession,
    setEntrypoint,
    pinSession,
    newSession,
  } = useProjects()
  const navigate = useNavigate()
  const [collapsed, setCollapsed] = useState(false)
  const ids = sessions.map((session) => session.id)
  const { sensors, onDragEnd } = useSortableList(ids, onReorder)
  const name = pinned ? "Pinned" : checkoutLabel(path, projectPath, projectId)
  const group = useSortable({ id: sortId, disabled: !showHeader || pinned })
  // The PR card keys off the group's real checkout — the project root for the
  // root group (empty path), else the worktree — so a root project on a feature
  // branch parks its card too, not only worktrees.
  const checkout = path || projectPath
  const pullsOpen = useSyncExternalStore(subscribePullsCard, () => isPullsOpen(checkout))

  const select = (id: string) => {
    activateSession(projectId, id)
    // From the settings screen this returns to the terminal; on the project
    // route it is a no-op.
    navigate(`/projects/${projectId}`)
  }

  return (
    <div
      ref={group.setNodeRef}
      style={dragStyle(group.transform, group.transition)}
      className={cn(
        "flex flex-col gap-1.5",
        group.isDragging && "pointer-events-none relative z-10 rounded-lg bg-sidebar shadow-md",
      )}
    >
      {showHeader && (
        <SessionGroupHeader
          name={name}
          pinned={pinned}
          collapsed={collapsed}
          isDragging={group.isDragging}
          providers={providers}
          activatorRef={group.setActivatorNodeRef}
          // The pinned block is never dragged, and dnd-kit answers a disabled
          // sortable with aria-disabled + aria-roledescription="draggable" —
          // which would announce a working collapse button as a dead handle.
          activatorProps={pinned ? {} : { ...group.attributes, ...group.listeners }}
          onToggle={() => setCollapsed((current) => !current)}
          onNewSession={(kind) => newSession(projectId, kind, path)}
        />
      )}
      <div
        aria-hidden={collapsed}
        {...(collapsed ? { inert: "" } : {})}
        className={cn(
          "grid transition-[grid-template-rows,opacity] duration-200 ease-out",
          collapsed
            ? "pointer-events-none grid-rows-[0fr] opacity-0"
            : "grid-rows-[1fr] opacity-100",
        )}
      >
        <div className="flex min-h-0 flex-col gap-1.5 overflow-hidden">
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            modifiers={[verticalAxis, withinList]}
            onDragEnd={onDragEnd}
          >
            <SortableContext items={ids} strategy={verticalListSortingStrategy}>
              <div className="flex flex-col gap-1.5">
                {sessions.map((session) => (
                  <SessionCard
                    key={session.id}
                    session={session}
                    path={projectPath}
                    // Resolved here rather than in the card: the parent can be a
                    // session in another project, which only the workspace-wide
                    // state knows about.
                    origin={sessionOrigin(workspace, session)}
                    active={session.id === activeId}
                    onSelect={() => select(session.id)}
                    onClose={() => onClose(session)}
                    onRename={(label) => renameSession(projectId, session.id, label)}
                    onSetEntrypoint={(entrypoint) =>
                      setEntrypoint(projectId, session.id, entrypoint)
                    }
                    onPin={(pinned) => pinSession(projectId, session.id, pinned)}
                    onOpenTerminal={(cwd) => newSession(projectId, "shell", cwd)}
                    onPulls={onPulls}
                    delegateGroups={delegateGroups}
                  />
                ))}
              </div>
            </SortableContext>
          </DndContext>
          {/* The pinned block spans every checkout, so no single pull request
              belongs under it — the card stays with the worktree's own block. */}
          {!pinned && pullsOpen && (
            <PullRequestCard
              path={checkout}
              active={pullsActive}
              onSelect={onPulls}
              onClose={onClosePulls}
            />
          )}
        </div>
      </div>
    </div>
  )
}
