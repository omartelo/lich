import { useEffect, useMemo, useState } from "react"
import { useNavigate } from "react-router-dom"
import { Folder, FolderX, GitBranch, MessageSquareText, TriangleAlert } from "lucide-react"
import { useProjects } from "@/providers/projects"
import { useSettings } from "@/providers/settings"
import { useHotkey } from "@/lib/use-hotkey"
import {
  runningSessions,
  useSessionStatus,
  useSessionUnread,
} from "@/lib/session/use-session-status"
import { SessionStatusIcon } from "@/components/sidebar/SessionStatusIcon"
import {
  filterPalette,
  historyAction,
  historyRows,
  nextTab,
  paletteGroups,
  paletteSessions,
  paletteTabCount,
  PALETTE_TABS,
  rankSessions,
  rowKey,
  type PaletteHistory,
  type PaletteMessage,
  type PaletteRow,
  type PaletteSession,
  type PaletteTab,
} from "@/lib/session/command-palette"
import { useTranscriptSearch } from "@/lib/session/use-transcript-search"
import { toast } from "sonner"
import { PickerDialog, PickerEmpty, PickerGroup, PickerRow } from "@/components/common/PickerDialog"
import { agoLabel } from "@/lib/ago"
import { displayPath } from "@/lib/paths"
import { isSessionKind } from "@/lib/session/sessions"
import type { ClosedSession, Project, RecentProject } from "@/lib/api-types"
import { ProjectService, Store } from "@/lib/rpc"
import { cn, errorText } from "@/lib/utils"

// CommandPalette is the app-wide quick switcher: one shortcut (Ctrl/Cmd+K by
// default, rebindable in Settings) to jump to any session across every project,
// to a project open or closed, or to what was said inside a session — reachable
// from anywhere, unlike the tab strip which only shows the active project's
// sessions. Mounted once at the app root; it renders nothing until opened.
//
// The trigger is caught in the window capture phase (like the other global
// hotkeys) so it beats the shell binding it shadows; while open, focus is
// trapped in the dialog and keys never reach the terminal.
export function CommandPalette() {
  const { projects, sessions, activateSession, openRecent, resumeClosedSession } = useProjects()
  const { hotkeys } = useSettings()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const [tab, setTab] = useState<PaletteTab>("All")
  const [selected, setSelected] = useState(0)
  const [closed, setClosed] = useState<RecentProject[]>([])
  const [parked, setParked] = useState<ClosedSession[]>([])
  const [branches, setBranches] = useState<Readonly<Record<string, string>>>({})
  const [running, setRunning] = useState<ReadonlySet<string>>(new Set())
  const [missing, setMissing] = useState<ReadonlySet<string>>(new Set())

  useHotkey(hotkeys.commandPalette, () => {
    setOpen((v) => !v)
    setQuery("")
    setTab("All")
    setSelected(0)
  })

  // The closed projects and the parked sessions both live in the store, not in
  // the workspace state, so they are fetched per opening — anything closed or
  // reopened since the last one moves in or out of these lists. Their
  // directories are read with them: a project whose folder moved is relocated
  // rather than reopened, and a session whose checkout is gone says so.
  //
  // The branches are asked for in one batch rather than by subscribing each row
  // to the git poller a live card uses: that poll is three calls per path per
  // second, and this list is long and on screen for as long as it takes to type.
  useEffect(() => {
    if (!open) {
      return
    }
    let live = true
    void Promise.all([Store.RecentProjects(), Store.ClosedSessions()]).then(
      ([recentRows, parkedRows]) => {
        const recents = recentRows ?? []
        const history = parkedRows ?? []
        if (!live) {
          return
        }
        setClosed(recents)
        setParked(history)
        // Asked for after the rows are up: what git and the filesystem say is
        // what a row says about itself, and a failed check must not cost the
        // palette its entries.
        const paths = history.map((row) => row.path).filter(Boolean)
        void ProjectService.Missing([...recents.map((row) => row.path), ...paths]).then((gone) => {
          if (live) {
            setMissing(new Set(gone ?? []))
          }
        })
        void ProjectService.BranchesOf(paths).then((named) => {
          if (live) {
            setBranches(named ?? {})
          }
        })
      },
    )
    return () => {
      live = false
    }
  }, [open])

  // Forgetting drops the row from the list in place rather than closing the
  // palette: the whole point of the action is that there are usually several of
  // them, left by worktrees removed outside lich.
  const forget = (session: PaletteHistory) => {
    setParked((rows) => rows.filter((row) => row.id !== session.id))
    void Store.ForgetSession(session.id).catch((error: unknown) => {
      toast.error(`Could not forget ${session.label}: ${errorText(error)}`)
    })
  }

  const flat = useMemo(() => paletteSessions(projects, sessions), [projects, sessions])
  // Which sessions hold a turn, sampled once per opening rather than
  // subscribed to. The palette is mounted for the whole app's life, so a
  // subscription would re-render it for every status report with nothing on
  // screen; and an order that moved under the cursor while a row is being
  // aimed at costs more than the freshness is worth — the same reason the
  // transcript hits are listed last.
  useEffect(() => {
    if (open) {
      setRunning(new Set(runningSessions(flat.map((session) => session.sessionId))))
    }
  }, [open])
  const all = useMemo(() => rankSessions(flat, running), [flat, running])
  const history = useMemo(() => historyRows(parked, branches, missing), [parked, branches, missing])
  const results = useMemo(
    () => filterPalette(query, all, projects, closed, history),
    [query, all, projects, closed, history],
  )
  // What was said inside the sessions, not just their names. It arrives after
  // the name-matched groups (it is a disk read behind a debounce), so it is
  // listed last and never moves a row the user is already aiming at.
  const messages = useTranscriptSearch(query, all, open)
  const groups = useMemo(() => paletteGroups(tab, results, messages), [tab, results, messages])
  const counts = useMemo(
    () => PALETTE_TABS.map((t) => paletteTabCount(t, results, messages)),
    [results, messages],
  )
  // The groups are what is rendered; this is the same rows flattened, so an
  // index is a position in the list the arrow keys walk.
  const rows = useMemo(() => groups.flatMap((g) => g.rows), [groups])
  const sections = useMemo(() => {
    let offset = 0
    return groups.map((group) => {
      const section = { group, offset }
      offset += group.rows.length
      return section
    })
  }, [groups])
  const total = rows.length

  useEffect(() => setSelected(0), [query, tab])
  const active = Math.min(selected, Math.max(0, total - 1))

  const openProject = (projectId: string, sessionId?: string) => {
    navigate(`/projects/${projectId}`)
    if (sessionId) {
      activateSession(projectId, sessionId)
    }
    close()
  }

  const runRow = (row: PaletteRow) => {
    switch (row.kind) {
      case "session":
        openProject(row.session.projectId, row.session.sessionId)
        return
      case "message":
        openProject(row.message.projectId, row.message.sessionId)
        return
      case "project":
        openProject(row.project.id)
        return
      // Reopening navigates on its own once the project is adopted, and asks
      // for the new directory first when the stored one is gone — either way
      // the palette has nothing left to show.
      case "closed":
        close()
        void openRecent(row.project)
        return
      // A resume navigates on its own, so the palette closes with it. Forgetting
      // does not: it drops one row and leaves the list up, because a workspace
      // with one stale row usually has several.
      case "history":
        if (historyAction(row.session) === "forget") {
          forget(row.session)
          return
        }
        close()
        void resumeClosedSession(row.session)
        return
    }
  }

  const runIndex = (index: number) => {
    const row = rows[index]
    if (row) {
      runRow(row)
    }
  }

  const close = () => {
    setOpen(false)
    setQuery("")
    setTab("All")
    setSelected(0)
  }

  // What Enter does with the row the cursor is on. Every row but one opens
  // something; the exception is a history row whose checkout is gone, which can
  // only be dropped — and the hint bar has to say so before it is pressed.
  const actionHint = (() => {
    const row = rows[active]
    return row?.kind === "history" ? historyAction(row.session) : "open"
  })()

  const onInputKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "ArrowDown") {
      event.preventDefault()
      // From `active`, not from `selected`: a list that shrank under the cursor
      // leaves the raw index past its end, and stepping off that one moves
      // nothing the user can see.
      setSelected(Math.min(active + 1, total - 1))
    } else if (event.key === "ArrowUp") {
      event.preventDefault()
      setSelected(Math.max(active - 1, 0))
    } else if (event.key === "Enter") {
      event.preventDefault()
      runIndex(active)
    } else if (event.key === "Tab") {
      // The dialog traps focus and there is nothing else in it worth tabbing to,
      // so Tab walks the filters instead of the focus ring.
      event.preventDefault()
      setTab((t) => nextTab(t, event.shiftKey ? -1 : 1))
    }
  }

  return (
    <PickerDialog
      open={open}
      onOpenChange={(next) => (next ? setOpen(true) : close())}
      title="Command palette"
      placeholder="Jump to a session, project or something said…"
      searchLabel="Search sessions and projects"
      resultsLabel="Results"
      query={query}
      onQueryChange={setQuery}
      onKeyDown={onInputKeyDown}
      actionHint={actionHint}
      filters={<FilterTabs tab={tab} counts={counts} onPick={setTab} />}
    >
      {total === 0 ? (
        // Two different empties, deliberately not sharing a sentence: one says
        // the query found nothing, the other says nothing has ever been closed.
        // The second is the only place the retention rule is stated, which is
        // where it belongs — the moment somebody wonders what this remembers.
        tab === "History" && parked.length === 0 ? (
          <PickerEmpty>
            <span className="block text-foreground">Nothing closed yet</span>
            <span className="mx-auto mt-2 block max-w-[44ch] leading-relaxed">
              Close a session and it waits here — its branch, its agent and its conversation — until
              its worktree is removed.
            </span>
          </PickerEmpty>
        ) : (
          <PickerEmpty>
            No matches for <span className="font-mono text-foreground/80">{query.trim()}</span>
            {tab !== "All" && <> in {tab.toLowerCase()}</>}
          </PickerEmpty>
        )
      ) : (
        sections.map(({ group, offset }) => (
          <PickerGroup
            key={group.label}
            label={group.label}
            trailing={
              group.total > group.rows.length ? `${group.rows.length} of ${group.total}` : undefined
            }
          >
            {group.rows.map((row, i) => (
              <ListRow
                key={rowKey(row)}
                row={row}
                sessionCount={
                  row.kind === "project" ? (sessions[row.project.id]?.sessions.length ?? 0) : 0
                }
                missing={row.kind === "closed" && missing.has(row.project.path)}
                selected={offset + i === active}
                onSelect={() => setSelected(offset + i)}
                onRun={() => runIndex(offset + i)}
              />
            ))}
          </PickerGroup>
        ))
      )}
    </PickerDialog>
  )
}

// FilterTabs narrows the list to one kind of hit. A tab with no match is dimmed
// rather than dropped: a row of tabs that reshuffles as the query changes costs
// more to aim at than the space it saves.
function FilterTabs({
  tab,
  counts,
  onPick,
}: {
  tab: PaletteTab
  counts: readonly (number | null)[]
  onPick: (tab: PaletteTab) => void
}) {
  return (
    <div role="tablist" aria-label="Filter results" className="flex flex-wrap items-center gap-1">
      {PALETTE_TABS.map((name, i) => {
        const count = counts[i]
        const current = name === tab
        return (
          <button
            key={name}
            type="button"
            role="tab"
            aria-selected={current}
            // The input keeps the focus, so typing continues straight after a
            // tab is clicked.
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => onPick(name)}
            className={cn(
              "inline-flex items-baseline gap-1.5 rounded-md px-2 py-1 text-[0.8125rem] outline-none",
              current
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-accent/50",
              !current && count === 0 && "opacity-40",
            )}
          >
            {name}
            {count !== null && count !== undefined && (
              <span className="font-mono text-[0.625rem] opacity-70">{count}</span>
            )}
          </button>
        )
      })}
    </div>
  )
}

function ListRow({
  row,
  sessionCount,
  missing,
  selected,
  onSelect,
  onRun,
}: {
  row: PaletteRow
  sessionCount: number
  /** Closed projects only: the stored directory is gone, so the row relocates. */
  missing: boolean
  selected: boolean
  onSelect: () => void
  onRun: () => void
}) {
  switch (row.kind) {
    case "session":
      return (
        <SessionRow session={row.session} selected={selected} onSelect={onSelect} onRun={onRun} />
      )
    case "message":
      return (
        <MessageRow message={row.message} selected={selected} onSelect={onSelect} onRun={onRun} />
      )
    case "project":
      return (
        <ProjectRow
          project={row.project}
          sessionCount={sessionCount}
          selected={selected}
          onSelect={onSelect}
          onRun={onRun}
        />
      )
    case "closed":
      return (
        <ClosedProjectRow
          project={row.project}
          missing={missing}
          selected={selected}
          onSelect={onSelect}
          onRun={onRun}
        />
      )
    case "history":
      return (
        <HistoryRow session={row.session} selected={selected} onSelect={onSelect} onRun={onRun} />
      )
  }
}

// A closed session names itself the way its card did — the provider mark, the
// label, the project and the checkout — plus the two facts a card never needed:
// the branch, because a worktree keeps the name it was created with while the
// branch moves on, and when it was closed, because that is what turns a list of
// old work into one somebody can find something in.
//
// The provider mark carries no status ring: nothing is running in a closed
// session, so the ring has nothing to report, and its absence is what tells a
// history row from a live one at a glance. That is also why the unread flag is
// a plain false — an unread turn is a turn somebody has not looked at yet, and
// closing the session is looking at it.
function HistoryRow({
  session,
  selected,
  onSelect,
  onRun,
}: {
  session: PaletteHistory
  selected: boolean
  onSelect: () => void
  onRun: () => void
}) {
  const closedAt = agoLabel(session.closedAt)
  return (
    <PickerRow selected={selected} onSelect={onSelect} onRun={onRun}>
      <SessionStatusIcon
        kind={isSessionKind(session.kind) ? session.kind : "claude"}
        status={null}
        unread={false}
      />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm">{session.label}</span>
        <span className="flex min-w-0 items-center gap-1.5 font-mono text-xs text-muted-foreground">
          <span className="shrink-0 text-foreground/70">{session.projectName}</span>
          <span className="shrink-0 opacity-45">·</span>
          {/* The checkout being gone replaces the branch rather than sitting
              beside it: there is no branch to read off a directory that is not
              there, and it is the same absence that makes the row unresumable. */}
          {session.gone ? (
            <span className="flex shrink-0 items-center gap-1 text-amber-500">
              <TriangleAlert className="size-3 shrink-0" />
              checkout gone
            </span>
          ) : (
            session.branch && (
              <span className="flex shrink-0 items-center gap-1">
                <GitBranch className="size-3 shrink-0" />
                {session.branch}
              </span>
            )
          )}
          {(session.gone || session.branch) && <span className="shrink-0 opacity-45">·</span>}
          <span className={cn("truncate", session.gone && "opacity-60")}>
            {displayPath(session.path)}
          </span>
        </span>
      </span>
      {closedAt && (
        <span className="shrink-0 font-mono text-[0.625rem] tabular-nums text-muted-foreground">
          {closedAt}
        </span>
      )}
    </PickerRow>
  )
}

function SessionRow({
  session,
  selected,
  onSelect,
  onRun,
}: {
  session: PaletteSession
  selected: boolean
  onSelect: () => void
  onRun: () => void
}) {
  const status = useSessionStatus(session.sessionId)
  const unread = useSessionUnread(session.sessionId)
  return (
    <PickerRow selected={selected} onSelect={onSelect} onRun={onRun}>
      <SessionStatusIcon kind={session.kind} status={status} unread={unread} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm">{session.label}</span>
        <span className="truncate font-mono text-xs text-muted-foreground">
          <span className="text-foreground/70">{session.projectName}</span> · {session.path}
        </span>
      </span>
    </PickerRow>
  )
}

// A message hit leads with what was said and names the session under it: the
// snippet is why the row is here, but the session is what the row opens.
function MessageRow({
  message,
  selected,
  onSelect,
  onRun,
}: {
  message: PaletteMessage
  selected: boolean
  onSelect: () => void
  onRun: () => void
}) {
  return (
    <PickerRow selected={selected} onSelect={onSelect} onRun={onRun}>
      <MessageSquareText className="size-4 shrink-0 text-muted-foreground" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm">{message.snippet}</span>
        <span className="truncate font-mono text-xs text-muted-foreground">
          <span className="text-foreground/70">{message.label}</span> · {message.projectName}
        </span>
      </span>
      {message.count > 1 && (
        <span className="shrink-0 font-mono text-[0.625rem] text-muted-foreground">
          {message.count} matches
        </span>
      )}
    </PickerRow>
  )
}

function ProjectRow({
  project,
  sessionCount,
  selected,
  onSelect,
  onRun,
}: {
  project: Project
  sessionCount: number
  selected: boolean
  onSelect: () => void
  onRun: () => void
}) {
  return (
    <PickerRow selected={selected} onSelect={onSelect} onRun={onRun}>
      <Folder className="size-4 shrink-0 text-muted-foreground" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm">{project.name}</span>
        <span className="truncate font-mono text-xs text-muted-foreground">{project.path}</span>
      </span>
      <span className="shrink-0 font-mono text-[0.625rem] text-muted-foreground">
        {sessionCount} {sessionCount === 1 ? "session" : "sessions"}
      </span>
    </PickerRow>
  )
}

// A closed project has no session to count, so the slot the open ones use for
// theirs names what Enter does with this one instead — which is not the same
// thing once the directory has moved: that row asks for the new one first.
function ClosedProjectRow({
  project,
  missing,
  selected,
  onSelect,
  onRun,
}: {
  project: Project
  missing: boolean
  selected: boolean
  onSelect: () => void
  onRun: () => void
}) {
  const Icon = missing ? FolderX : Folder
  return (
    <PickerRow selected={selected} onSelect={onSelect} onRun={onRun}>
      <Icon className="size-4 shrink-0 text-muted-foreground" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm">{project.name}</span>
        <span className="truncate font-mono text-xs text-muted-foreground">{project.path}</span>
      </span>
      <span className="shrink-0 font-mono text-[0.625rem] text-muted-foreground">
        {missing ? "relocate" : "reopen"}
      </span>
    </PickerRow>
  )
}
