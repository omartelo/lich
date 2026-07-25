import { useSyncExternalStore } from "react"
import { DndContext, closestCenter } from "@dnd-kit/core"
import { SortableContext, verticalListSortingStrategy } from "@dnd-kit/sortable"
import { useSortableList, verticalAxis } from "@/lib/use-sortable-list"
import { baseName } from "@/lib/paths"
import type { Session } from "@/lib/session/sessions"
import { SessionCard } from "./SessionCard"
import { PullRequestCard } from "./PullRequestCard"
import { isPullsOpen, subscribePullsCard } from "@/lib/pulls-card-store"

interface SessionGroupProps {
  // "" for the project's own root, else the worktree checkout path.
  path: string
  sessions: Session[]
  projectPath: string
  projectName: string
  activeId: string
  // A divider label is drawn only when the sidebar holds more than one group; a
  // lone project with no worktrees keeps its old flat, header-less list.
  showHeader: boolean
  // Commits a new order for this group's sessions; a drag can only ever produce
  // one, since each group owns an isolated DndContext.
  onReorder: (ids: string[]) => void
  onSelect: (id: string) => void
  onClose: (session: Session) => void
  onRename: (id: string, label: string) => void
  onOpenTerminal: (cwd: string) => void
  // The worktree's pull-request entry: opens the Pulls screen for this branch.
  // pullsActive marks it when that screen is showing this group's PR. Rendered
  // only for worktree groups (a truthy path).
  pullsActive: boolean
  onPulls: () => void
  onClosePulls: () => void
}

// SessionGroup renders one worktree's sessions under a static divider titled
// with the worktree folder name; the branch stays on each card. The title reads
// the group's own checkout path, never a session's live cwd, so a `cd` deeper
// into the tree never re-buckets the group. The isolated DndContext is what
// confines a drag to reordering within the group.
export function SessionGroup({
  path,
  sessions,
  projectPath,
  projectName,
  activeId,
  showHeader,
  onReorder,
  onSelect,
  onClose,
  onRename,
  onOpenTerminal,
  pullsActive,
  onPulls,
  onClosePulls,
}: SessionGroupProps) {
  const ids = sessions.map((session) => session.id)
  const { sensors, onDragEnd } = useSortableList(ids, onReorder)
  const name = path ? baseName(path) : projectName
  // The PR card keys off the group's real checkout — the project root for the
  // root group (empty path), else the worktree — so a root project on a feature
  // branch parks its card too, not only worktrees.
  const checkout = path || projectPath
  const pullsOpen = useSyncExternalStore(subscribePullsCard, () => isPullsOpen(checkout))

  return (
    <div className="flex flex-col gap-1.5">
      {showHeader && (
        <div className="flex items-center gap-2 px-1 pb-0.5 pt-1.5">
          <span className="min-w-0 truncate text-[0.65rem] font-semibold uppercase tracking-wider text-muted-foreground/70">
            {name}
          </span>
          <span className="h-px flex-1 bg-border" />
        </div>
      )}
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        modifiers={[verticalAxis]}
        onDragEnd={onDragEnd}
      >
        <SortableContext items={ids} strategy={verticalListSortingStrategy}>
          <div className="flex flex-col gap-1.5">
            {sessions.map((session) => (
              <SessionCard
                key={session.id}
                session={session}
                path={projectPath}
                active={session.id === activeId}
                onSelect={() => onSelect(session.id)}
                onClose={() => onClose(session)}
                onRename={(label) => onRename(session.id, label)}
                onOpenTerminal={onOpenTerminal}
                onPulls={onPulls}
              />
            ))}
          </div>
        </SortableContext>
      </DndContext>
      {pullsOpen && (
        <PullRequestCard
          path={checkout}
          active={pullsActive}
          onSelect={onPulls}
          onClose={onClosePulls}
        />
      )}
    </div>
  )
}
