import {DndContext, closestCenter} from "@dnd-kit/core"
import {SortableContext, verticalListSortingStrategy} from "@dnd-kit/sortable"
import {useGitStatus} from "@/lib/useGitStatus"
import {useSortableList, verticalAxis} from "@/lib/use-sortable-list"
import {baseName} from "@/lib/paths"
import type {Session} from "@/lib/sessions"
import {SessionCard} from "./SessionCard"

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
}

// SessionGroup renders one worktree's sessions under a static divider label
// (folder name + branch). The label reads the group's own checkout path, never a
// session's live cwd, so a `cd` deeper into the tree never restyles or re-buckets
// the group. The isolated DndContext is what confines a drag to reordering
// within the group.
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
}: SessionGroupProps) {
  const git = useGitStatus(path || projectPath)
  const ids = sessions.map((session) => session.id)
  const {sensors, onDragEnd} = useSortableList(ids, onReorder)
  const name = path ? baseName(path) : projectName

  return (
    <div className="flex flex-col gap-1.5">
      {showHeader && (
        <div className="flex items-center gap-2 px-1 pb-0.5 pt-1.5">
          <span className="flex min-w-0 items-center gap-1.5 text-[0.65rem] font-semibold uppercase tracking-wider text-muted-foreground/70">
            <span className="truncate">{name}</span>
            {git?.branch && (
              <span className="shrink-0 font-mono font-normal normal-case tracking-normal text-muted-foreground/90">
                · {git.branch}
              </span>
            )}
          </span>
          <span className="h-px flex-1 bg-border"/>
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
              />
            ))}
          </div>
        </SortableContext>
      </DndContext>
    </div>
  )
}
