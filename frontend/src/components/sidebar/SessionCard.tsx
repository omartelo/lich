import { useEffect, useRef, useState } from "react"
import type { KeyboardEvent } from "react"
import {
  ArrowDown,
  AtSign,
  GitBranch,
  GitPullRequestArrow,
  Pencil,
  Pin,
  PinOff,
  Terminal,
  TriangleAlert,
  X,
} from "lucide-react"
import { useSortable } from "@dnd-kit/sortable"
import { cn } from "@/lib/utils"
import { dragStyle } from "@/lib/use-sortable-list"
import { displayPath } from "@/lib/paths"
import type { Session } from "@/lib/session/sessions"
import { peerMention, peerName } from "@/lib/session/peer-name"
import { useSessionStatus, useSessionStatusAge } from "@/lib/session/use-session-status"
import { useSessionCwd } from "@/lib/session/use-session-cwd"
import { useSessionAgent } from "@/lib/session/use-session-agent"
import { useSessionTool } from "@/lib/session/use-session-tool"
import { toolGlyph } from "@/lib/session/tool-glyph"
import { useGitStatus } from "@/lib/git/use-git-status"
import { baseReadout } from "@/lib/git/base-status"
import { usePullRequest } from "@/lib/pulls/use-pull-request"
import { CloseButton } from "@/components/common/CloseButton"
import { DiffStat } from "@/components/DiffStat"
import { SessionStatusIcon } from "./SessionStatusIcon"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuGroup,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSeparator,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import { Terminal as TerminalService } from "@/lib/rpc"
import type { MentionGroup } from "@/lib/session/mention-targets"
import { bracketedPaste } from "@/lib/terminal/bracketed-paste"
import { requestTerminalFocus } from "@/lib/terminal/focus-request"

interface SessionCardProps {
  session: Session
  path: string
  active: boolean
  onSelect: () => void
  onClose: () => void
  onRename: (label: string) => void
  // Pin the card to the head of the list, or unpin it. A pinned card offers no
  // close affordance at all — unpinning is the way back to closing it.
  onPin: (pinned: boolean) => void
  // Open a shell session rooted at this card's shown directory. Wired only for
  // agent sessions, so the user can drop into a terminal in the worktree the
  // agent is working in without cd-ing there by hand.
  onOpenTerminal: (cwd: string) => void
  // Open the Pulls screen for this session's worktree, parking its PR card.
  onPulls: () => void
  // Claude sessions this one can be pointed at, grouped by project. Only the
  // card whose terminal is on screen offers them — the mention is written at
  // that terminal's prompt, so any other card would be writing somewhere the
  // user cannot see.
  mentionGroups: MentionGroup[]
}

// The card itself is the drag grip for reordering the list — no separate handle.
export function SessionCard({
  session,
  path,
  active,
  onSelect,
  onClose,
  onRename,
  onPin,
  onOpenTerminal,
  onPulls,
  mentionGroups,
}: SessionCardProps) {
  const pinned = !!session.pinned
  const pathRef = useRef<HTMLSpanElement>(null)
  const [pathOverflow, setPathOverflow] = useState(false)
  const [editing, setEditing] = useState(false)
  // Processing state reported by the lich Claude Code hook, drawn as a ring
  // around the provider icon: a spinning ring while Claude produces output,
  // solid emerald once its turn ends, amber when it is blocked on the user.
  // null before the first report, and whenever the hook reports a state with
  // no indicator (see toSessionStatus) — then the icon shows ringless.
  const status = useSessionStatus(session.id)
  // How long that state has lasted, beside the ring: with five agents running,
  // the bells all look alike and the one blocked longest is the one to answer
  // first. "" for the states that have no clock (see useSessionStatusAge).
  const age = useSessionStatusAge(session.id)
  // The provider CLI live inside the PTY right now — a hand-run `claude` or
  // `codex` in a shell session puts that provider's mark on the card while it
  // runs; null falls back to the session's own kind.
  const agent = useSessionAgent(session.id)
  // The tool the turn is running right now, reported by the provider's pre-tool
  // hook: null outside a tool call, which is what keeps the card its usual size
  // whenever nothing is happening in it.
  const tool = useSessionTool(session.id)
  const ToolGlyph = tool && toolGlyph(tool.name)
  // The live working directory the backend's cwd watcher reports ("" until it
  // does): a `cd` in the terminal moves the card with it. Falls back to the
  // session's static start path — a worktree session lives in its own checkout,
  // so that path (not the project's) is the fallback. Git status and the PR
  // badge follow whatever is shown, so they reflect the directory's repo.
  const liveCwd = useSessionCwd(session.id)
  const shownPath = liveCwd || session.path || path
  const git = useGitStatus(shownPath)
  const pr = usePullRequest(shownPath, git?.branch ?? "", git?.head ?? "")
  // How the checkout stands against the branch it merges into: null — the case
  // nearly all day — draws nothing at all.
  const base = baseReadout(git?.base ?? null)
  // Renaming disables the drag: the sensor would otherwise claim the pointer
  // before the input could be clicked into or its text selected.
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: session.id,
    disabled: editing,
  })

  // Write the target's roster name at this session's own prompt and hand the
  // cursor back. lich stops here: the Claude reading that prompt is the one that
  // addresses the other session, with the tool it already has.
  const mention = (name: string) => {
    void TerminalService.Write(session.id, bracketedPaste(peerMention(name)))
    requestTerminalFocus(session.id)
  }

  const canMention = active && session.kind === "claude" && mentionGroups.length > 0

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
      style={dragStyle(transform, transition)}
      className={cn("relative", isDragging && "pointer-events-none z-10 rounded-lg shadow-md")}
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
                  // biome-ignore lint/a11y/noAutofocus: the rename field replaces the label only once editing starts.
                  autoFocus
                  defaultValue={session.label}
                  onFocus={(event) => event.currentTarget.select()}
                  onClick={(event) => event.stopPropagation()}
                  onKeyDown={onEditKeyDown}
                  onBlur={(event) => commit(event.currentTarget.value)}
                  className={cn(
                    "w-full rounded-sm bg-transparent text-sm font-medium text-foreground outline-none ring-1 ring-accent-foreground/30",
                    pinned ? "pr-6" : "pr-11",
                  )}
                />
              ) : (
                <span
                  className={cn(
                    "flex w-full min-w-0 items-center gap-1.5",
                    pinned ? "pr-6" : "pr-11",
                  )}
                >
                  <SessionStatusIcon kind={agent ?? session.kind} status={status} />
                  {age && (
                    <span className="shrink-0 text-xs tabular-nums text-muted-foreground">
                      {age}
                    </span>
                  )}
                  <span className="truncate text-sm font-medium text-foreground">
                    {session.label}
                  </span>
                </span>
              )}
              {/* Under the label, so the card reads name → what it is doing →
                  where. It is the only row that comes and goes with the turn,
                  which is the cost of putting it where the eye already is. */}
              {tool && (
                <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
                  {ToolGlyph && <ToolGlyph className="size-3 shrink-0" />}
                  <span className="shrink-0 font-medium text-foreground">{tool.name}</span>
                  {tool.detail && (
                    <>
                      <span className="shrink-0 opacity-50">·</span>
                      <span className="truncate font-mono">{tool.detail}</span>
                    </>
                  )}
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
                {`\u200e${displayPath(shownPath)}`}
              </span>
              {git?.branch && (
                <span className="flex w-full items-center justify-between gap-2 text-xs text-muted-foreground">
                  <span className="flex min-w-0 items-center gap-1">
                    <GitBranch className="size-3 shrink-0" />
                    <span className="truncate">{git.branch}</span>
                  </span>
                  <span className="flex shrink-0 items-center gap-1.5">
                    {/* Base standing first, then the PR, then the diff: the
                        cluster reads outward from the branch it qualifies. */}
                    {base && (
                      <span
                        className={cn(
                          "flex items-center gap-0.5 tabular-nums",
                          base.kind === "conflict" && "text-amber-500",
                        )}
                      >
                        {base.kind === "conflict" ? (
                          <TriangleAlert className="size-3 shrink-0" />
                        ) : (
                          <ArrowDown className="size-3 shrink-0" />
                        )}
                        {base.count}
                      </span>
                    )}
                    {pr && (
                      <span
                        role="button"
                        aria-label={`View pull request #${pr.number}`}
                        onClick={(event) => {
                          event.stopPropagation()
                          onPulls()
                        }}
                        className="flex items-center gap-1 rounded-sm transition-colors hover:text-foreground"
                      >
                        <GitPullRequestArrow className="size-3 shrink-0" />#{pr.number}
                      </span>
                    )}
                    {git.files > 0 && <DiffStat added={git.added} deleted={git.deleted} />}
                  </span>
                </span>
              )}
            </div>
            {/* A pinned card keeps its pin on screen — it is both the state's
                only mark and the way to undo it — and shows no × at all: closing
                is what the pin withholds. */}
            <span className="absolute right-2 top-2 flex items-center gap-1">
              <span
                role="button"
                aria-label={pinned ? `Unpin ${session.label}` : `Pin ${session.label}`}
                onClick={(event) => {
                  event.stopPropagation()
                  onPin(!pinned)
                }}
                className={cn(
                  "flex size-4 shrink-0 items-center justify-center rounded transition-opacity hover:bg-foreground/15",
                  pinned ? "text-foreground" : "opacity-0 group-hover:opacity-100",
                )}
              >
                <Pin className={cn("size-3", pinned && "fill-current")} />
              </span>
              {!pinned && (
                <CloseButton
                  label={`Close ${session.label}`}
                  onClick={(event) => {
                    event.stopPropagation()
                    onClose()
                  }}
                />
              )}
            </span>
          </ContextMenuTrigger>
          <TooltipContent
            side="right"
            className="max-w-xs border border-border bg-card text-foreground"
          >
            <div className="flex flex-col gap-1.5">
              <span className="font-medium">{session.label}</span>
              <span className="break-all font-mono text-muted-foreground">{shownPath}</span>
              {/* The name this session answers to when another Claude Code
                  session messages it. Built from the start path, not the shown
                  one: a `cd` in the terminal never renames a running session. */}
              {session.kind === "claude" && (
                <span className="flex items-center gap-1 font-mono text-muted-foreground">
                  <AtSign className="size-3 shrink-0" />
                  {peerName(session.path || path, session.id)}
                </span>
              )}
              {git?.branch && (
                <span className="flex flex-wrap items-center gap-2 text-muted-foreground">
                  <span className="flex items-center gap-1">
                    <GitBranch className="size-3 shrink-0" />
                    {git.branch}
                  </span>
                  {pr && (
                    <span className="flex items-center gap-1">
                      <GitPullRequestArrow className="size-3 shrink-0" />#{pr.number}
                    </span>
                  )}
                  {git.files > 0 && (
                    <span className="flex items-center gap-1.5">
                      <DiffStat added={git.added} deleted={git.deleted} />
                    </span>
                  )}
                </span>
              )}
              {/* The base standing spelled out — the card itself has room for a
                  glyph and a number, and this is the only place the branch it
                  measures against is named. */}
              {base?.behind && <span className="text-muted-foreground">{base.behind}</span>}
              {base?.conflict && (
                <span className="flex items-center gap-1 text-amber-500">
                  <TriangleAlert className="size-3 shrink-0" />
                  {base.conflict}
                </span>
              )}
              {base && base.paths.length > 0 && (
                <span className="flex flex-col gap-0.5 pl-4 text-muted-foreground">
                  {base.paths.map((file) => (
                    <span key={file} className="break-all font-mono">
                      {file}
                    </span>
                  ))}
                  {base.more > 0 && <span>+{base.more} more</span>}
                </span>
              )}
            </div>
          </TooltipContent>
        </Tooltip>
        <ContextMenuContent>
          {canMention && (
            <ContextMenuSub>
              <ContextMenuSubTrigger>
                <AtSign />
                Mention session
              </ContextMenuSubTrigger>
              <ContextMenuSubContent>
                {mentionGroups.map((group) => (
                  <ContextMenuGroup key={group.projectId}>
                    {/* The project only earns a heading when there is more than
                        one to tell apart. */}
                    {mentionGroups.length > 1 && (
                      <ContextMenuLabel>{group.projectName}</ContextMenuLabel>
                    )}
                    {group.targets.map((target) => (
                      <ContextMenuItem key={target.id} onClick={() => mention(target.name)}>
                        <span className="truncate">{target.label}</span>
                        <span className="ml-auto pl-3 font-mono text-xs text-muted-foreground">
                          {target.name}
                        </span>
                      </ContextMenuItem>
                    ))}
                  </ContextMenuGroup>
                ))}
              </ContextMenuSubContent>
            </ContextMenuSub>
          )}
          <ContextMenuItem onClick={() => setEditing(true)}>
            <Pencil />
            Rename
          </ContextMenuItem>
          <ContextMenuItem onClick={() => onPin(!pinned)}>
            {pinned ? <PinOff /> : <Pin />}
            {pinned ? "Unpin" : "Pin"}
          </ContextMenuItem>
          {session.kind !== "shell" && (
            <ContextMenuItem onClick={() => onOpenTerminal(shownPath)}>
              <Terminal />
              Open Terminal
            </ContextMenuItem>
          )}
          <ContextMenuItem onClick={onPulls}>
            <GitPullRequestArrow />
            Pull request
          </ContextMenuItem>
          {!pinned && (
            <>
              <ContextMenuSeparator />
              <ContextMenuItem variant="destructive" onClick={onClose}>
                <X />
                Close session
              </ContextMenuItem>
            </>
          )}
        </ContextMenuContent>
      </ContextMenu>
    </div>
  )
}
