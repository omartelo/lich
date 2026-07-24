import {useEffect, useRef, useState} from "react"
import type {KeyboardEvent} from "react"
import {GitBranch, GitPullRequestArrow, Pencil, Terminal, X} from "lucide-react"
import {useSortable} from "@dnd-kit/sortable"
import {CSS} from "@dnd-kit/utilities"
import {cn} from "@/lib/utils"
import {displayPath} from "@/lib/paths"
import {type Session} from "@/lib/sessions"
import {useSessionStatus} from "@/lib/useSessionStatus"
import {useSessionCwd} from "@/lib/useSessionCwd"
import {useSessionAgent} from "@/lib/useSessionAgent"
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
  // Open a shell session rooted at this card's shown directory. Wired only for
  // agent sessions, so the user can drop into a terminal in the worktree the
  // agent is working in without cd-ing there by hand.
  onOpenTerminal: (cwd: string) => void
}

// The card itself is the drag grip for reordering the list — no separate handle.
export function SessionCard({
                              session,
                              path,
                              active,
                              onSelect,
                              onClose,
                              onRename,
                              onOpenTerminal,
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
  // The provider CLI live inside the PTY right now — a hand-run `claude` in a
  // shell session puts Claude's mark on the card while it runs; null falls
  // back to the session's own kind.
  const agent = useSessionAgent(session.id)
  // The live working directory the backend's cwd watcher reports ("" until it
  // does): a `cd` in the terminal moves the card with it. Falls back to the
  // session's static start path — a worktree session lives in its own checkout,
  // so that path (not the project's) is the fallback. Git status and the PR
  // badge follow whatever is shown, so they reflect the directory's repo.
  const liveCwd = useSessionCwd(session.id)
  const shownPath = liveCwd || session.path || path
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
      className={cn(
        "relative",
        isDragging && "pointer-events-none z-10 rounded-lg shadow-md",
      )}
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
                      "group relative flex w-full flex-col items-start gap-0.5 rounded-md px-2.5 py-2 text-left transition-colors hover:bg-accent/60",
                      active && "bg-accent text-accent-foreground",
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
                  <SessionStatusIcon kind={agent ?? session.kind} status={status}/>
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
                        className="flex items-center gap-1 rounded-sm transition-colors hover:text-foreground"
                      >
                        <GitPullRequestArrow className="size-3 shrink-0"/>
                        #{pr.number}
                      </span>
                    )}
                    {git.files > 0 && (
                      <DiffStat added={git.added} deleted={git.deleted}/>
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
          {session.kind !== "shell" && (
            <ContextMenuItem onClick={() => onOpenTerminal(shownPath)}>
              <Terminal/>
              Open Terminal
            </ContextMenuItem>
          )}
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
