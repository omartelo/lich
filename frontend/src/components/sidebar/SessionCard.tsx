import {useEffect, useRef, useState} from "react"
import type {KeyboardEvent} from "react"
import {GitBranch, GitPullRequestArrow, Pencil, X} from "lucide-react"
import {useSortable} from "@dnd-kit/sortable"
import {CSS} from "@dnd-kit/utilities"
import {cn} from "@/lib/utils"
import {displayPath} from "@/lib/paths"
import {type Session} from "@/lib/sessions"
import {useSessionStatus} from "@/lib/useSessionStatus"
import {useGitStatus} from "@/lib/useGitStatus"
import {usePullRequest} from "@/lib/usePullRequest"
import {System} from "@/lib/rpc"
import {DiffStat} from "@/components/DiffStat"
import {SessionStatusIcon} from "./SessionStatusIcon"
import {Tooltip, TooltipContent, TooltipTrigger} from "@/components/ui/tooltip"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"

interface SessionCardProps {
  session: Session
  path: string
  active: boolean
  onSelect: () => void
  onClose: () => void
  onRename: (label: string) => void
}

// SessionCard is one session entry: a card showing the session label, the
// session's working directory, and that directory's git branch with a diff
// badge (when it is a repo), with a close button on hover. The card itself is
// the drag grip for reordering the list.
export function SessionCard({
                              session,
                              path,
                              active,
                              onSelect,
                              onClose,
                              onRename,
                            }: SessionCardProps) {
  const pathRef = useRef<HTMLSpanElement>(null)
  const [pathOverflow, setPathOverflow] = useState(false)
  const [editing, setEditing] = useState(false)
  // Processing state reported by the lich Claude Code hook, drawn as a ring
  // around the provider icon: a spinning ring while Claude produces output,
  // solid emerald once its turn ends, amber when it is blocked on the user.
  // null before the first report, and whenever the hook reports a state with
  // no indicator (see toSessionStatus) — then the icon shows ringless.
  const status = useSessionStatus(session.id)
  // A worktree session lives in its own checkout: show that path and poll its
  // git status, so the badge reflects the worktree's branch, not the project's.
  const shownPath = session.path || path
  const git = useGitStatus(shownPath)
  const pr = usePullRequest(shownPath, git?.branch ?? "")
  // Renaming disables the drag: the sensor would otherwise claim the pointer
  // before the input could be clicked into or its text selected.
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({id: session.id, disabled: editing})

  // Commit the edited label: keep the old one if it is blank or unchanged.
  const commit = (value: string) => {
    setEditing(false)
    const label = value.trim()
    if (label && label !== session.label) {
      onRename(label)
    }
  }

  const onEditKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") {
      commit(event.currentTarget.value)
    } else if (event.key === "Escape") {
      setEditing(false)
    }
  }

  // Fade the left (path start) only when the tail can't fit, so a path that
  // fits keeps its "~" crisp — matching how terminals hint at hidden prefix.
  useEffect(() => {
    const el = pathRef.current
    if (!el) return
    const measure = () => setPathOverflow(el.scrollWidth > el.clientWidth)
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(el)
    return () => observer.disconnect()
  }, [shownPath])

  return (
    // The sortable node is this wrapper rather than the card button itself, so
    // the drag never has to thread a ref through the context-menu and tooltip
    // triggers that render it.
    <div
      ref={setNodeRef}
      style={{transform: CSS.Transform.toString(transform), transition}}
      className={cn("relative", isDragging && "z-10 opacity-60")}
      {...attributes}
      {...listeners}
    >
      <ContextMenu>
        <Tooltip>
          <ContextMenuTrigger
            render={
              <TooltipTrigger
                render={
                  <button
                    type="button"
                    onClick={onSelect}
                    className={cn(
                      "group relative flex w-full flex-col items-start gap-0.5 rounded-lg border border-border/60 bg-card px-3 py-3 text-left transition-colors hover:bg-accent/60",
                      active &&
                      "border-accent-foreground/20 bg-accent text-accent-foreground",
                    )}
                  />
                }
              />
            }
          >
            <div className="flex w-full min-w-0 flex-col space-y-2">
              {editing ? (
                <input
                  autoFocus
                  defaultValue={session.label}
                  onFocus={(event) => event.currentTarget.select()}
                  onClick={(event) => event.stopPropagation()}
                  onKeyDown={onEditKeyDown}
                  onBlur={(event) => commit(event.currentTarget.value)}
                  className="w-full rounded-sm bg-transparent pr-5 text-sm font-medium text-foreground outline-none ring-1 ring-accent-foreground/30"
                />
              ) : (
                <span className="flex w-full min-w-0 items-center gap-1.5 pr-5">
                  <SessionStatusIcon kind={session.kind} status={status}/>
                  <span className="truncate text-sm font-medium text-foreground">
                    {session.label}
                  </span>
                </span>
              )}
              {/* rtl anchors the tail (project folder) to the right so overflow is
                clipped on the left; the leading LRM keeps "~/" in logical order
                instead of letting bidi push it to the end. */}
              <span
                ref={pathRef}
                dir="rtl"
                className={cn(
                  "block max-w-full overflow-hidden whitespace-nowrap text-left font-mono text-xs text-muted-foreground",
                  pathOverflow &&
                  "[mask-image:linear-gradient(to_right,transparent,black_1.25rem)]",
                )}
              >
              {"\u200e" + displayPath(shownPath)}
            </span>
              {git?.branch && (
                <span className="flex w-full items-center justify-between gap-2 text-xs text-muted-foreground">
                <span className="flex min-w-0 items-center gap-1">
                  <GitBranch className="size-3 shrink-0"/>
                  <span className="truncate">{git.branch}</span>
                </span>
                  <span className="flex shrink-0 items-center gap-1.5">
                    {pr && (
                      <span
                        role="button"
                        aria-label={`Open pull request #${pr.number} on GitHub`}
                        onClick={(event) => {
                          event.stopPropagation()
                          void System.OpenExternal(pr.url)
                        }}
                        className="flex items-center gap-1 px-1 py-0.5 bg-muted-foreground/10 rounded transition-colors hover:bg-accent hover:text-accent-foreground"
                      >
                        <GitPullRequestArrow className="size-3 shrink-0"/>
                        #{pr.number}
                      </span>
                    )}
                    {git.files > 0 && (
                      <span className="flex items-center gap-1 px-1 py-0.5 bg-muted-foreground/10 rounded">
                        <DiffStat added={git.added} deleted={git.deleted}/>
                      </span>
                    )}
                  </span>
              </span>
              )}
            </div>
            <span
              role="button"
              aria-label={`Close ${session.label}`}
              onClick={(event) => {
                event.stopPropagation()
                onClose()
              }}
              className="absolute right-2 top-2 flex size-4 items-center justify-center rounded opacity-0 transition-opacity hover:bg-foreground/15 group-hover:opacity-100"
            >
            <X className="size-3"/>
          </span>
          </ContextMenuTrigger>
          <TooltipContent
            side="right"
            className="max-w-xs border border-border bg-card text-foreground"
          >
            <div className="flex flex-col gap-1.5">
              <span className="font-medium">{session.label}</span>
              <span className="break-all font-mono text-muted-foreground">{shownPath}</span>
              {git?.branch && (
                <span className="flex flex-wrap items-center gap-2 text-muted-foreground">
                  <span className="flex items-center gap-1">
                    <GitBranch className="size-3 shrink-0"/>
                    {git.branch}
                  </span>
                  {pr && (
                    <span className="flex items-center gap-1">
                      <GitPullRequestArrow className="size-3 shrink-0"/>#{pr.number}
                    </span>
                  )}
                  {git.files > 0 && (
                    <span className="flex items-center gap-1.5">
                      <DiffStat added={git.added} deleted={git.deleted}/>
                    </span>
                  )}
                </span>
              )}
            </div>
          </TooltipContent>
        </Tooltip>
        <ContextMenuContent>
          <ContextMenuItem onClick={() => setEditing(true)}>
            <Pencil/>
            Rename
          </ContextMenuItem>
          <ContextMenuSeparator/>
          <ContextMenuItem variant="destructive" onClick={onClose}>
            <X/>
            Close session
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
    </div>
  )
}
