import { useEffect, useMemo, useState } from "react"
import { useNavigate } from "react-router-dom"
import { Folder, MessageSquareText } from "lucide-react"
import { useProjects } from "@/providers/projects"
import { useSettings } from "@/providers/settings"
import { useHotkey } from "@/lib/use-hotkey"
import { useSessionStatus } from "@/lib/session/use-session-status"
import { SessionStatusIcon } from "@/components/sidebar/SessionStatusIcon"
import {
  filterPalette,
  nextTab,
  paletteGroups,
  paletteSessions,
  paletteTabCount,
  PALETTE_TABS,
  rowKey,
  type PaletteMessage,
  type PaletteRow,
  type PaletteSession,
  type PaletteTab,
} from "@/lib/session/command-palette"
import { useTranscriptSearch } from "@/lib/session/use-transcript-search"
import { PickerDialog, PickerEmpty, PickerGroup, PickerRow } from "@/components/common/PickerDialog"
import type { Project, RecentProject } from "@/lib/api-types"
import { Store } from "@/lib/rpc"
import { cn } from "@/lib/utils"

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
  const { projects, sessions, activateSession, openRecent } = useProjects()
  const { hotkeys } = useSettings()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const [tab, setTab] = useState<PaletteTab>("All")
  const [selected, setSelected] = useState(0)
  const [closed, setClosed] = useState<RecentProject[]>([])

  useHotkey(hotkeys.commandPalette, () => {
    setOpen((v) => !v)
    setQuery("")
    setTab("All")
    setSelected(0)
  })

  // The closed projects live in the store, not in the projects state, so they
  // are fetched per opening — a project closed or reopened since the last one
  // moves in or out of this list.
  useEffect(() => {
    if (!open) {
      return
    }
    let live = true
    void Store.RecentProjects().then((rows) => {
      if (live) {
        setClosed(rows ?? [])
      }
    })
    return () => {
      live = false
    }
  }, [open])

  const all = useMemo(() => paletteSessions(projects, sessions), [projects, sessions])
  const results = useMemo(
    () => filterPalette(query, all, projects, closed),
    [query, all, projects, closed],
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
      // Reopening navigates on its own once the project is adopted, and drops
      // the row instead when its directory is gone — either way the palette has
      // nothing left to show.
      case "closed":
        close()
        void openRecent(row.project)
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
      actionHint="open"
      filters={<FilterTabs tab={tab} counts={counts} onPick={setTab} />}
    >
      {total === 0 ? (
        <PickerEmpty>
          No matches for <span className="font-mono text-foreground/80">{query.trim()}</span>
          {tab !== "All" && <> in {tab.toLowerCase()}</>}
        </PickerEmpty>
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
  selected,
  onSelect,
  onRun,
}: {
  row: PaletteRow
  sessionCount: number
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
          selected={selected}
          onSelect={onSelect}
          onRun={onRun}
        />
      )
  }
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
  return (
    <PickerRow selected={selected} onSelect={onSelect} onRun={onRun}>
      <SessionStatusIcon kind={session.kind} status={status} />
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
// theirs names what Enter does with this one instead.
function ClosedProjectRow({
  project,
  selected,
  onSelect,
  onRun,
}: {
  project: Project
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
      <span className="shrink-0 font-mono text-[0.625rem] text-muted-foreground">reopen</span>
    </PickerRow>
  )
}
