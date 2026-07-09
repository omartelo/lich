import {useEffect, useRef, useState} from "react"
import type {KeyboardEvent} from "react"
import {GitBranch, Pencil, X} from "lucide-react"
import {cn} from "@/lib/utils"
import {displayPath} from "@/lib/paths"
import {type Session} from "@/lib/sessions"
import {useGitStatus} from "@/lib/useGitStatus"
import {Tooltip, TooltipContent, TooltipTrigger} from "@/components/ui/tooltip"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
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
// badge (when it is a repo), with a close button on hover.
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
  // A worktree session lives in its own checkout: show that path and poll its
  // git status, so the badge reflects the worktree's branch, not the project's.
  const shownPath = session.path || path
  const git = useGitStatus(shownPath)

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
              <span className="w-full truncate pr-5 text-sm font-medium text-foreground">
              {session.label}
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
                {git.files > 0 && (
                  <span className="flex shrink-0 items-center gap-1 px-1 py-0.5 bg-muted-foreground/10 rounded">
                    <span className="font-medium text-sky-600 dark:text-sky-400">
                      +{git.added}
                    </span>
                    <span className="font-medium text-pink-600 dark:text-pink-400">
                      -{git.deleted}
                    </span>
                  </span>
                )}
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
        <TooltipContent>{shownPath}</TooltipContent>
      </Tooltip>
      <ContextMenuContent>
        <ContextMenuItem onClick={() => setEditing(true)}>
          <Pencil/>
          Rename
        </ContextMenuItem>
        <ContextMenuItem variant="destructive" onClick={onClose}>
          <X/>
          Close session
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}
