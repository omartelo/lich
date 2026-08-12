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
  paletteSessions,
  type PaletteMessage,
  type PaletteSession,
} from "@/lib/session/command-palette"
import { useTranscriptSearch } from "@/lib/session/use-transcript-search"
import { PickerDialog, PickerEmpty, PickerGroup, PickerRow } from "@/components/common/PickerDialog"
import type { Project } from "@/lib/api-types"

// CommandPalette is the app-wide quick switcher: one shortcut (Ctrl/Cmd+K by
// default, rebindable in Settings) to jump to any session across every project,
// or to a project — reachable from anywhere, unlike the tab strip which only
// shows the active project's sessions. Mounted once at the app root; it renders
// nothing until opened.
//
// The trigger is caught in the window capture phase (like the other global
// hotkeys) so it beats the shell binding it shadows; while open, focus is
// trapped in the dialog and keys never reach the terminal.
export function CommandPalette() {
  const { projects, sessions, activateSession } = useProjects()
  const { hotkeys } = useSettings()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const [selected, setSelected] = useState(0)

  useHotkey(hotkeys.commandPalette, () => {
    setOpen((v) => !v)
    setQuery("")
    setSelected(0)
  })

  const all = useMemo(() => paletteSessions(projects, sessions), [projects, sessions])
  const results = useMemo(() => filterPalette(query, all, projects), [query, all, projects])
  // What was said inside the sessions, not just their names. It arrives after
  // the two name-matched groups (it is a disk read behind a debounce), so it is
  // listed last and never moves a row the user is already aiming at.
  const messages = useTranscriptSearch(query, all, open)
  const messageOffset = results.sessions.length + results.projects.length
  const total = messageOffset + messages.length

  useEffect(() => setSelected(0), [query])
  const active = Math.min(selected, Math.max(0, total - 1))

  const openProject = (projectId: string, sessionId?: string) => {
    navigate(`/projects/${projectId}`)
    if (sessionId) {
      activateSession(projectId, sessionId)
    }
    close()
  }

  const runIndex = (index: number) => {
    if (index < results.sessions.length) {
      const s = results.sessions[index]
      if (s) {
        openProject(s.projectId, s.sessionId)
      }
      return
    }
    if (index >= messageOffset) {
      const m = messages[index - messageOffset]
      if (m) {
        openProject(m.projectId, m.sessionId)
      }
      return
    }
    const p = results.projects[index - results.sessions.length]
    if (p) {
      openProject(p.id)
    }
  }

  const close = () => {
    setOpen(false)
    setQuery("")
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
    }
  }

  return (
    <PickerDialog
      open={open}
      onOpenChange={(next) => (next ? setOpen(true) : close())}
      title="Command palette"
      placeholder="Jump to a session or project…"
      searchLabel="Search sessions and projects"
      resultsLabel="Results"
      query={query}
      onQueryChange={setQuery}
      onKeyDown={onInputKeyDown}
      actionHint="open"
    >
      {total === 0 ? (
        <PickerEmpty>
          No matches for <span className="font-mono text-foreground/80">{query.trim()}</span>
        </PickerEmpty>
      ) : (
        <>
          {results.sessions.length > 0 && (
            <PickerGroup label="Sessions">
              {results.sessions.map((session, i) => (
                <SessionRow
                  key={session.sessionId}
                  session={session}
                  selected={i === active}
                  onSelect={() => setSelected(i)}
                  onRun={() => runIndex(i)}
                />
              ))}
            </PickerGroup>
          )}
          {results.projects.length > 0 && (
            <PickerGroup label="Projects">
              {results.projects.map((project, j) => {
                const index = results.sessions.length + j
                return (
                  <ProjectRow
                    key={project.id}
                    project={project}
                    sessionCount={sessions[project.id]?.sessions.length ?? 0}
                    selected={index === active}
                    onSelect={() => setSelected(index)}
                    onRun={() => runIndex(index)}
                  />
                )
              })}
            </PickerGroup>
          )}
          {messages.length > 0 && (
            <PickerGroup label="Messages">
              {messages.map((message, k) => {
                const index = messageOffset + k
                return (
                  <MessageRow
                    key={message.sessionId}
                    message={message}
                    selected={index === active}
                    onSelect={() => setSelected(index)}
                    onRun={() => runIndex(index)}
                  />
                )
              })}
            </PickerGroup>
          )}
        </>
      )}
    </PickerDialog>
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
