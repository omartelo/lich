import { useEffect, useRef, useState } from "react"
import type { KeyboardEvent } from "react"
import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  CircleQuestionMark,
  CornerDownLeft,
  GitBranch,
  GitPullRequestArrow,
  Inbox,
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
import { useSessionStatus, useSessionStatusAge } from "@/lib/session/use-session-status"
import { useSessionCwd } from "@/lib/session/use-session-cwd"
import { useSessionAgent } from "@/lib/session/use-session-agent"
import { useSessionRelay } from "@/lib/session/use-session-relay"
import { useSessionInbox } from "@/lib/session/use-session-inbox"
import { useSessionTool } from "@/lib/session/use-session-tool"
import { toolGlyph } from "@/lib/session/tool-glyph"
import { useGitStatus } from "@/lib/git/use-git-status"
import { baseReadout } from "@/lib/git/base-status"
import { usePullRequest } from "@/lib/pulls/use-pull-request"
import { CloseButton } from "@/components/common/CloseButton"
import { DiffStat } from "@/components/DiffStat"
import { SessionStatusIcon } from "./SessionStatusIcon"
import { SessionTooltip } from "./SessionTooltip"
import { Tooltip, TooltipTrigger } from "@/components/ui/tooltip"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import { Terminal as TerminalService } from "@/lib/rpc"
import type { DelegateGroup } from "@/lib/session/delegate-targets"
import { delegatePrompt, delegateWorktreePrompt } from "@/lib/session/delegate-prompt"
import { bracketedPaste } from "@/lib/terminal/bracketed-paste"
import { requestTerminalFocus } from "@/lib/terminal/focus-request"
import { SessionTargetPicker } from "./SessionTargetPicker"

interface SessionCardProps {
  session: Session
  path: string
  // The session this one was opened from, named as it is called now; "" for a
  // session nobody delegated, which is most of them (sessionOrigin).
  origin: string
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
  // Sessions this one can hand work to, grouped by project. Only the card
  // whose terminal is on screen offers them — the request is written at that
  // terminal's prompt, so any other card would be writing somewhere the user
  // cannot see.
  delegateGroups: DelegateGroup[]
}

// The card itself is the drag grip for reordering the list — no separate handle.
export function SessionCard({
  session,
  path,
  origin,
  active,
  onSelect,
  onClose,
  onRename,
  onPin,
  onOpenTerminal,
  onPulls,
  delegateGroups,
}: SessionCardProps) {
  const pinned = !!session.pinned
  const pathRef = useRef<HTMLSpanElement>(null)
  const [pathOverflow, setPathOverflow] = useState(false)
  const [editing, setEditing] = useState(false)
  const [delegatePickerOpen, setDelegatePickerOpen] = useState(false)
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
  // The request this session has open with another, reported by the relay when
  // a message lands in a PTY and cleared when it is answered. null the rest of
  // the time, which is nearly always.
  const relay = useSessionRelay(session.id)
  // How many results this session has waiting in the relay's inbox: results of
  // tasks it delegated, uncollected. Zero — the usual case — draws nothing.
  const inbox = useSessionInbox(session.id)
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

  // Write the request at this session's own prompt and hand the cursor back.
  // lich stops here: what it types is a request, and the agent reading it
  // decides whether to reach for the tool or the command (delegatePrompt).
  const delegate = (label: string) => {
    void TerminalService.Write(session.id, bracketedPaste(delegatePrompt(session.kind, label)))
    requestTerminalFocus(session.id)
  }
  const delegateWorktree = () => {
    void TerminalService.Write(session.id, bracketedPaste(delegateWorktreePrompt(session.kind)))
    requestTerminalFocus(session.id)
  }

  // Every provider can delegate, and no live target is required: the picker's
  // pinned row delegates into a fresh worktree session, which is most useful
  // exactly when there is nobody else to hand work to yet.
  const canDelegate = active

  // The picker is only rendered while the card can delegate, so losing that
  // unmounts it — and an open flag left behind would spring the dialog back up
  // unasked the moment the card qualifies again. The card can stop being the
  // active one without a click on it (the palette hotkey is caught in the
  // window's capture phase, and a session link jumps straight to another card),
  // so this is reachable with the picker on screen.
  useEffect(() => {
    if (!canDelegate) {
      setDelegatePickerOpen(false)
    }
  }, [canDelegate])

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
              {/* One line, five rungs: an open request, then a session blocked
                  on the user, then results waiting to be collected, then the
                  tool, then where the session came from. A request in flight
                  explains the whole turn — a card working because another
                  session asked it to, or one stalled waiting on a card
                  elsewhere in the list. A block outranks the rest for the
                  reason it needs words at all: the amber ring differs from the
                  emerald one by hue alone, so nothing else on the card says the
                  session wants an answer. The inbox sits under those and over
                  the tool: mid-turn the live tool is the news, and the count
                  takes the rung when the card goes quiet — the same rule the
                  relay's own nudge follows. The origin is last precisely
                  because it is never news: it says something that has been true
                  since the card was created, so it surfaces only once the card
                  is quiet, which is when somebody scanning the sidebar is
                  working out where a card came from. Only one rung ever draws,
                  so the card grows by one row at most. */}
              {relay ? (
                <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
                  {relay.direction === "out" ? (
                    <ArrowRight className="size-3 shrink-0" />
                  ) : (
                    <ArrowLeft className="size-3 shrink-0" />
                  )}
                  {relay.peer ? (
                    <span className="truncate font-medium text-foreground">{relay.peer}</span>
                  ) : (
                    // Not a session, so not a label: the other end is the `lich`
                    // command run from a script or a shell (docs/cli.md).
                    <span className="truncate italic">command line</span>
                  )}
                </span>
              ) : status === "waiting" ? (
                <span className="flex w-full min-w-0 items-center gap-1 text-xs">
                  <CircleQuestionMark className="size-3 shrink-0 text-amber-500" />
                  <span className="truncate font-medium text-amber-500">Waiting on you</span>
                </span>
              ) : status !== "busy" && inbox > 0 ? (
                <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
                  <Inbox className="size-3 shrink-0" />
                  <span className="truncate font-medium text-foreground">
                    {inbox === 1 ? "1 result ready" : `${inbox} results ready`}
                  </span>
                </span>
              ) : tool ? (
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
              ) : (
                // Quieter than the rungs above it, on purpose: muted throughout
                // and at normal weight, where an open request puts its peer in
                // text-foreground. The word "from" earns its place — an arrow
                // and a name alone read as traffic happening now, which is the
                // confusion this rung exists to end. Not a link either: the
                // parent may be closed, and a dead link is worse than a
                // sentence.
                origin && (
                  <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
                    <CornerDownLeft className="size-3 shrink-0" />
                    <span className="truncate">from {origin}</span>
                  </span>
                )
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
          <SessionTooltip session={session} path={path} />
        </Tooltip>
        <ContextMenuContent>
          {canDelegate && (
            <ContextMenuItem onClick={() => setDelegatePickerOpen(true)}>
              <ArrowRight />
              Delegate to session…
            </ContextMenuItem>
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
      {canDelegate && (
        <SessionTargetPicker
          open={delegatePickerOpen}
          onOpenChange={setDelegatePickerOpen}
          groups={delegateGroups}
          onPick={(target) => delegate(target.label)}
          onPickWorktree={delegateWorktree}
        />
      )}
    </div>
  )
}
