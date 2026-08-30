import { useEffect, useRef, useState } from "react"
import type { KeyboardEvent } from "react"
import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  CircleQuestionMark,
  Copy,
  CornerDownLeft,
  FolderCode,
  FolderOpen,
  GitBranch,
  GitPullRequestArrow,
  Inbox,
  Pencil,
  Columns2,
  Pin,
  PinOff,
  Play,
  Shield,
  Terminal,
  TriangleAlert,
  X,
} from "lucide-react"
import { useSortable } from "@dnd-kit/sortable"
import { toast } from "sonner"
import { cn, errorText } from "@/lib/utils"
import { dragStyle } from "@/lib/use-sortable-list"
import { displayPath } from "@/lib/paths"
import type { Session } from "@/lib/session/sessions"
import {
  useSessionStatus,
  useSessionStatusAge,
  useSessionUnread,
  useSessionWaitingReason,
} from "@/lib/session/use-session-status"
import { useSessionCwd } from "@/lib/session/use-session-cwd"
import { useSessionAgent } from "@/lib/session/use-session-agent"
import { useSessionRelay } from "@/lib/session/use-session-relay"
import { useSessionInbox } from "@/lib/session/use-session-inbox"
import { useSessionTool } from "@/lib/session/use-session-tool"
import { toolGlyph } from "@/lib/session/tool-glyph"
import { toolLabel } from "@/lib/session/tool-label"
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
import { System, Terminal as TerminalService } from "@/lib/rpc"
import { queuePaste } from "@/lib/terminal/paste-queue"
import type { DelegateGroup } from "@/lib/session/delegate-targets"
import { delegatePrompt, delegateWorktreePrompt } from "@/lib/session/delegate-prompt"
import { isWindows } from "@/lib/platform"
import { sendCommand } from "@/lib/session/send-command"
import { bracketedPaste } from "@/lib/terminal/bracketed-paste"
import { requestTerminalFocus } from "@/lib/terminal/focus-request"
import { useSessionIntent } from "@/lib/use-sidebar-intent"
import { useProjects } from "@/providers/projects"
import { SessionTargetPicker } from "./SessionTargetPicker"
import { EntrypointDialog } from "./EntrypointDialog"

interface SessionCardProps {
  session: Session
  path: string
  // The session this one was opened from, named as it is called now; "" for a
  // session nobody delegated, which is most of them (sessionOrigin).
  origin: string
  active: boolean
  // Whether this session is on the stage at all. It is what the menu entry
  // answers to, and it is true for a member of a wall that is parked — the block
  // this card is drawn in is the mark, so nothing is repeated on the card.
  onStage: boolean
  // Whether it is drawing in a pane *right now*. Never true for a member of a
  // parked wall, which is exactly the difference the block cannot show.
  showing: boolean
  // Put this card on the stage, or take it off when it is already there.
  onStageToggle: () => void
  onSelect: () => void
  onClose: () => void
  onRename: (label: string) => void
  // Pin the card to the head of the list, or unpin it. A pinned card offers no
  // close affordance at all — unpinning is the way back to closing it.
  onPin: (pinned: boolean) => void
  // Open a shell session rooted at this card's shown directory, and answer with
  // its id. The menu item is wired for agent sessions alone — the user dropping
  // into a terminal in the worktree the agent works in, without cd-ing there by
  // hand — but the id is what lets a terminal editor be launched in it.
  onOpenTerminal: (cwd: string) => string
  // Record the command this terminal opens into, "" to clear it back to a plain
  // shell. Offered on shell sessions alone: on a provider card the entrypoint is
  // the provider, and the store refuses one there anyway.
  onSetEntrypoint: (entrypoint: string) => void
  // Open the Pulls screen for this session's worktree, parking its PR card.
  onPulls: () => void
  // Whether the card can be dragged. False while the sidebar holds a filter,
  // where a drop would compute an order the store rejects wholesale.
  sortable: boolean
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
  onStage,
  showing,
  onStageToggle,
  onSelect,
  onClose,
  onRename,
  onPin,
  onOpenTerminal,
  onSetEntrypoint,
  onPulls,
  sortable,
  delegateGroups,
}: SessionCardProps) {
  // Read here rather than threaded down as a prop: the `lich send` line names
  // the project only when another session shares this card's label, and that is
  // a question about every open project — not about the one this card sits in.
  const { projects, sessions } = useProjects()
  const pinned = !!session.pinned
  const pathRef = useRef<HTMLSpanElement>(null)
  const [pathOverflow, setPathOverflow] = useState(false)
  const [editing, setEditing] = useState(false)
  const [delegatePickerOpen, setDelegatePickerOpen] = useState(false)
  const [entrypointOpen, setEntrypointOpen] = useState(false)
  // Processing state reported by the lich Claude Code hook, drawn as a ring
  // around the provider icon: a spinning ring while Claude produces output,
  // solid emerald once its turn ends, amber when it is blocked on the user.
  // null before the first report, and whenever the hook reports a state with
  // no indicator (see toSessionStatus) — then the icon shows ringless.
  const status = useSessionStatus(session.id)
  // Whether that state is news: a turn that finished while the user was
  // elsewhere, still unread. It fades out of the ring the moment this card is
  // the one being looked at.
  const unread = useSessionUnread(session.id)
  // How long that state has lasted, beside the ring: with five agents running,
  // the bells all look alike and the one blocked longest is the one to answer
  // first. "" for the states that have no clock (see useSessionStatusAge).
  const age = useSessionStatusAge(session.id)
  // What the session is blocked on, when its provider's event had words for it:
  // the line the user reads to decide whether this is the card to open. "" from
  // a provider that reports the block and nothing about it.
  const waitingReason = useSessionWaitingReason(session.id)
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
    disabled: editing || !sortable,
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

  // Open this card's checkout, the folder itself. The backend either launched a
  // GUI editor detached (empty reply) or handed back the command line for a
  // terminal editor: run that in a shell session at the checkout, the way the
  // files panel does for a single file.
  const openFolderInEditor = () => {
    void System.OpenFolderInEditor(shownPath)
      .then((command) => {
        if (command) {
          queuePaste(onOpenTerminal(shownPath), `${command}\n`)
        }
      })
      .catch((error) => toast.error(`Could not open the checkout: ${errorText(error)}`))
  }

  // The line another terminal — or another agent — hands this session work
  // with. The label is quoted for a shell on the way out (sendCommand), in the
  // spelling this machine's own shell reads: getting that right from memory is
  // exactly what goes wrong when the line is retyped.
  const copySendCommand = () => {
    const command = sendCommand(projects, sessions, session, isWindows)
    void navigator.clipboard.writeText(command).then(
      () => toast(`Copied: ${command}`),
      (error) => toast.error(`Could not copy the command: ${errorText(error)}`),
    )
  }

  // A worktree removed outside lich leaves its card behind, so both openers
  // report a checkout that is gone rather than launching at nothing.
  const openFolder = () => {
    void System.OpenFolder(shownPath).catch((error) =>
      toast.error(`Could not open the folder: ${errorText(error)}`),
    )
  }

  // Every provider can delegate, and no live target is required: the picker's
  // pinned row delegates into a fresh worktree session, which is most useful
  // exactly when there is nobody else to hand work to yet.
  //
  // A terminal cannot. Delegating writes the request at this card's own prompt
  // and hands the cursor back, and the thing reading a terminal's prompt is a
  // shell: it would run the line as a command, or fail on it. The declared kind
  // decides, never the live agent readout — a menu that appears and disappears
  // as an agent is started and quit by hand offers no action the user can rely
  // on being there.
  const canDelegate = active && session.kind !== "shell"

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

  // The shortcuts for this card's own actions. They aim at the active session,
  // which is this card, and they call the same handlers its context menu items
  // do — one behaviour, two ways in. Whether an action is offered at all is
  // decided where the shortcut is raised (App), so a chord the card cannot
  // honour never reaches here.
  useSessionIntent(session.id, (intent) => {
    switch (intent) {
      case "rename":
        setEditing(true)
        break
      case "close":
        onClose()
        break
      case "pin":
        onPin(!pinned)
        break
      case "terminal":
        onOpenTerminal(shownPath)
        break
      case "delegate":
        setDelegatePickerOpen(true)
        break
    }
  })

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
      // A card in flight floats over its neighbours, so it carries a background
      // of its own: the card's own fill is transparent until it is hovered or
      // active, and without one the labels underneath read straight through the
      // card being dragged.
      className={cn(
        "relative",
        isDragging &&
          "pointer-events-none z-10 rounded-lg bg-popover shadow-lg ring-1 ring-foreground/10",
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
                      // On screen, but not the pane the keyboard is in: one step
                      // down the same fill, never a second kind of mark.
                      showing && !active && "bg-accent/55",
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
                  <SessionStatusIcon kind={agent ?? session.kind} status={status} unread={unread} />
                  {age && (
                    <span className="shrink-0 text-xs tabular-nums text-muted-foreground">
                      {age}
                    </span>
                  )}
                  {/* Permanent state, so it sits with the status and the age
                      rather than on the line below, which is a ladder where one
                      rung draws at a time and every rung is news. Muted and
                      wordless: the tooltip carries what it means. */}
                  {session.sandboxed && (
                    <Shield
                      aria-label="Sandboxed"
                      className="size-3 shrink-0 text-muted-foreground"
                    />
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
                  {/* The question takes the whole line when there is one: the
                      amber glyph and the ring around the icon already say the
                      session is waiting, so spending the width on saying it
                      again would cost the card the only words on it the user
                      cannot already see. Not every provider has them (see
                      docs/hooks/session-state.md), and the generic line is what
                      those fall back to. */}
                  <span className="truncate font-medium text-amber-500">
                    {waitingReason || "Waiting on you"}
                  </span>
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
                  {/* The detail gives its width up first and the name only once
                      the detail has none left to give — which is what the lopsided
                      shrink factor buys. Both still shrink, so neither can push the
                      row past the card the way a name that refused to shrink did.
                      The separator travels inside the detail so it leaves with it,
                      instead of dangling after a truncated name. */}
                  <span className="min-w-0 truncate font-medium text-foreground">
                    {toolLabel(tool.name)}
                  </span>
                  {tool.detail && (
                    <span className="min-w-0 shrink-[9999] truncate font-mono">
                      <span className="opacity-50">·</span> {tool.detail}
                    </span>
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
          <ContextMenuItem onClick={copySendCommand}>
            <Copy />
            Copy send command
          </ContextMenuItem>
          {!active && (
            <ContextMenuItem onClick={onStageToggle}>
              <Columns2 />
              {onStage ? "Stop showing" : "Show beside"}
            </ContextMenuItem>
          )}
          <ContextMenuItem onClick={() => setEditing(true)}>
            <Pencil />
            Rename
          </ContextMenuItem>
          {session.kind === "shell" && (
            <ContextMenuItem onClick={() => setEntrypointOpen(true)}>
              <Play />
              Entrypoint…
            </ContextMenuItem>
          )}
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
          <ContextMenuItem onClick={openFolderInEditor}>
            <FolderCode />
            Open in editor
          </ContextMenuItem>
          <ContextMenuItem onClick={openFolder}>
            <FolderOpen />
            Open folder
          </ContextMenuItem>
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
      {session.kind === "shell" && (
        <EntrypointDialog
          open={entrypointOpen}
          onOpenChange={setEntrypointOpen}
          entrypoint={session.entrypoint ?? ""}
          cwd={displayPath(shownPath)}
          onSave={onSetEntrypoint}
        />
      )}
    </div>
  )
}
