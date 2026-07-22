import { useEffect, useMemo, useRef, useState } from "react"
import { Dialog as DialogPrimitive } from "@base-ui/react/dialog"
import { useNavigate } from "react-router-dom"
import { CornerDownLeft, Folder, Search } from "lucide-react"
import { useProjects } from "@/lib/projects"
import { useSettings } from "@/lib/settings"
import { isRecordingTarget, matchesCombo } from "@/lib/hotkeys"
import { useSessionStatus } from "@/lib/useSessionStatus"
import { SessionStatusIcon } from "@/components/sidebar/SessionStatusIcon"
import { filterPalette, paletteSessions, type PaletteSession } from "@/lib/command-palette"
import type { Project } from "@/lib/api-types"
import { cn } from "@/lib/utils"

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

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (isRecordingTarget(event)) {
        return
      }
      if (matchesCombo(event, hotkeys.commandPalette)) {
        event.preventDefault()
        event.stopPropagation()
        setOpen((v) => !v)
      }
    }
    window.addEventListener("keydown", onKey, true)
    return () => window.removeEventListener("keydown", onKey, true)
  }, [hotkeys])

  const all = useMemo(() => paletteSessions(projects, sessions), [projects, sessions])
  const results = useMemo(() => filterPalette(query, all, projects), [query, all, projects])
  const total = results.sessions.length + results.projects.length

  // Reset the cursor to the top whenever the visible set changes.
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
      setSelected((i) => Math.min(i + 1, total - 1))
    } else if (event.key === "ArrowUp") {
      event.preventDefault()
      setSelected((i) => Math.max(i - 1, 0))
    } else if (event.key === "Enter") {
      event.preventDefault()
      runIndex(active)
    }
  }

  return (
    <DialogPrimitive.Root open={open} onOpenChange={(next) => (next ? setOpen(true) : close())}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Backdrop className="fixed inset-0 z-50 bg-black/50 supports-backdrop-filter:backdrop-blur-xs data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0" />
        <DialogPrimitive.Popup className="fixed left-1/2 top-[14vh] z-50 flex max-h-[70vh] w-full max-w-[40rem] -translate-x-1/2 flex-col overflow-hidden rounded-xl border bg-popover text-popover-foreground shadow-2xl ring-1 ring-foreground/10 outline-none data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0">
          <DialogPrimitive.Title className="sr-only">Command palette</DialogPrimitive.Title>

          <div className="flex items-center gap-3 border-b px-4 py-3">
            <Search className="size-4 shrink-0 text-muted-foreground" />
            <input
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={onInputKeyDown}
              placeholder="Jump to a session or project…"
              aria-label="Search sessions and projects"
              autoComplete="off"
              spellCheck={false}
              className="flex-1 bg-transparent text-[0.9375rem] outline-none placeholder:text-muted-foreground"
            />
          </div>

          <div className="flex-1 overflow-y-auto p-1.5">
            {total === 0 ? (
              <div className="px-3 py-8 text-center text-sm text-muted-foreground">
                No matches for <span className="font-mono text-foreground/80">{query.trim()}</span>
              </div>
            ) : (
              <>
                {results.sessions.length > 0 && <GroupLabel>Sessions</GroupLabel>}
                {results.sessions.map((session, i) => (
                  <SessionRow
                    key={session.sessionId}
                    session={session}
                    selected={i === active}
                    onSelect={() => setSelected(i)}
                    onRun={() => runIndex(i)}
                  />
                ))}
                {results.projects.length > 0 && <GroupLabel>Projects</GroupLabel>}
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
              </>
            )}
          </div>

          <div className="flex items-center gap-4 border-t bg-black/10 px-4 py-2 text-xs text-muted-foreground">
            <Hint keys={["↑", "↓"]}>navigate</Hint>
            <Hint keys={["↵"]}>open</Hint>
            <Hint keys={["esc"]}>close</Hint>
          </div>
        </DialogPrimitive.Popup>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}

function GroupLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="px-3 pb-1 pt-3 text-[0.65625rem] font-semibold uppercase tracking-wider text-muted-foreground">
      {children}
    </div>
  )
}

interface RowProps {
  selected: boolean
  onSelect: () => void
  onRun: () => void
  children: React.ReactNode
}

function Row({ selected, onSelect, onRun, children }: RowProps) {
  const ref = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (selected) {
      ref.current?.scrollIntoView({ block: "nearest" })
    }
  }, [selected])
  return (
    <button
      ref={ref}
      type="button"
      aria-selected={selected}
      onMouseMove={onSelect}
      onClick={onRun}
      className={cn(
        "group flex w-full items-center gap-3 rounded-md px-3 py-2 text-left outline-none",
        selected ? "bg-accent text-accent-foreground" : "text-foreground",
      )}
    >
      {children}
      <CornerDownLeft
        className={cn("size-3.5 shrink-0 text-muted-foreground", selected ? "opacity-100" : "opacity-0")}
      />
    </button>
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
    <Row selected={selected} onSelect={onSelect} onRun={onRun}>
      <SessionStatusIcon kind={session.kind} status={status} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm">{session.label}</span>
        <span className="truncate font-mono text-xs text-muted-foreground">
          <span className="text-foreground/70">{session.projectName}</span> · {session.path}
        </span>
      </span>
    </Row>
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
    <Row selected={selected} onSelect={onSelect} onRun={onRun}>
      <Folder className="size-4 shrink-0 text-muted-foreground" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm">{project.name}</span>
        <span className="truncate font-mono text-xs text-muted-foreground">{project.path}</span>
      </span>
      <span className="shrink-0 font-mono text-[0.625rem] text-muted-foreground">
        {sessionCount} {sessionCount === 1 ? "session" : "sessions"}
      </span>
    </Row>
  )
}

function Hint({ keys, children }: { keys: string[]; children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="inline-flex gap-1">
        {keys.map((k) => (
          <kbd
            key={k}
            className="rounded border border-b-2 bg-muted px-1.5 py-0.5 font-mono text-[0.625rem] leading-none text-muted-foreground"
          >
            {k}
          </kbd>
        ))}
      </span>
      {children}
    </span>
  )
}
