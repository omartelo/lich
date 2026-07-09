import {useRef, useState} from "react"
import type {PointerEvent as ReactPointerEvent} from "react"
import {useMatch} from "react-router-dom"
import {Bot, GitBranch, Plus, Terminal} from "lucide-react"
import {toast} from "sonner"
import {Service as ProjectService} from "../../../bindings/github.com/omartelo/lich/internal/project"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {useProjects} from "@/lib/projects"
import {activeSessionId, sessionsOf, type Session} from "@/lib/sessions"
import {CloseWorktreeDialog, ForceRemoveWorktreeDialog} from "./CloseWorktreeDialog"
import {SessionCard} from "./SessionCard"
import {WorktreeDialog} from "./WorktreeDialog"
import {useGitStatus} from "@/lib/useGitStatus"

// Sidebar width bounds in rem, matching the Tailwind v4 spacing scale. State and
// storage stay in rem; the pointer drag delta arrives in CSS pixels and is
// converted with the 16px root font size Tailwind assumes.
const REM_PX = 16
const MIN_WIDTH_REM = 12
const MAX_WIDTH_REM = 30
const DEFAULT_WIDTH_REM = 15
const WIDTH_STORAGE_KEY = "lich.sidebar.width"

const clampWidth = (rem: number): number =>
  Math.min(MAX_WIDTH_REM, Math.max(MIN_WIDTH_REM, rem))

function readWidth(): number {
  const stored = Number(localStorage.getItem(WIDTH_STORAGE_KEY))
  return Number.isFinite(stored) && stored > 0
    ? clampWidth(stored)
    : DEFAULT_WIDTH_REM
}

// SessionSidebar lists the active project's sessions and can be drag-resized
// within a fixed pixel range. Width persists across restarts. It renders nothing
// when no project is active (Home, Settings), so it never competes with those
// screens.
//
// Resizing only changes this element's width; the terminal keeps its PTY in sync
// on its own via a ResizeObserver, so the sidebar does not need to know about it.
export function SessionSidebar() {
  const {
    projects,
    sessions,
    newSession,
    newWorktreeSession,
    closeSession,
    activateSession,
    renameSession,
  } = useProjects()
  const match = useMatch("/projects/:projectId")
  const projectId = match?.params.projectId
  const path = projects.find((p) => p.id === projectId)?.path ?? ""
  const git = useGitStatus(path)
  const [width, setWidth] = useState(readWidth)
  const dragRef = useRef<{ startX: number; startWidth: number } | null>(null)
  const [worktreeOpen, setWorktreeOpen] = useState(false)
  const [pendingClose, setPendingClose] = useState<Session | null>(null)
  const [pendingForce, setPendingForce] = useState<Session | null>(null)

  if (!projectId) {
    return null
  }

  const list = sessionsOf(sessions, projectId)
  const activeId = activeSessionId(sessions, projectId)

  const createWorktree = async (name: string, base: string, baseIsRemote: boolean) => {
    const wt = await ProjectService.CreateWorktree(path, projectId, name, base, baseIsRemote)
    if (wt) {
      newWorktreeSession(projectId, wt)
    }
    setWorktreeOpen(false)
  }

  const resumeWorktree = (wt: { name: string; path: string }) => {
    newWorktreeSession(projectId, wt)
    setWorktreeOpen(false)
  }

  // Closing a worktree session asks what to do with the checkout; regular
  // sessions close immediately.
  const requestClose = (session: Session) => {
    if (session.path) {
      setPendingClose(session)
      return
    }
    closeSession(projectId, session.id)
  }

  const keepAndClose = () => {
    if (pendingClose) {
      closeSession(projectId, pendingClose.id)
    }
    setPendingClose(null)
  }

  // Close first so the PTY running inside the worktree dies before git tries
  // to remove it. A refused removal surfaces as a toast; the checkout stays on
  // disk and reappears in the new-worktree picker.
  const closeAndRemove = (session: Session, force: boolean) => {
    closeSession(projectId, session.id)
    ProjectService.RemoveWorktree(path, session.path ?? "", force).catch(
      (err: unknown) => {
        toast.error(
          `Failed to remove worktree: ${err instanceof Error ? err.message : String(err)}`,
        )
      },
    )
  }

  const removeAndClose = async () => {
    const session = pendingClose
    setPendingClose(null)
    if (!session?.path) {
      return
    }
    // A dirty worktree needs a second confirmation before --force discards its
    // changes. A failed check falls through to the plain remove, whose own
    // refusal surfaces as a toast.
    const dirty = await ProjectService.WorktreeDirty(session.path).catch(
      () => false,
    )
    if (dirty) {
      setPendingForce(session)
      return
    }
    closeAndRemove(session, false)
  }

  const forceRemoveAndClose = () => {
    const session = pendingForce
    setPendingForce(null)
    if (session?.path) {
      closeAndRemove(session, true)
    }
  }

  const startDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault()
    dragRef.current = {startX: event.clientX, startWidth: width}
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  const onDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current
    if (!drag) {
      return
    }
    setWidth(clampWidth(drag.startWidth + (event.clientX - drag.startX) / REM_PX))
  }

  const endDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current
    if (!drag) {
      return
    }
    const finalWidth = clampWidth(
      drag.startWidth + (event.clientX - drag.startX) / REM_PX,
    )
    dragRef.current = null
    event.currentTarget.releasePointerCapture(event.pointerId)
    setWidth(finalWidth)
    localStorage.setItem(WIDTH_STORAGE_KEY, String(finalWidth))
  }

  return (
    <aside
      className="relative flex shrink-0 flex-col border-r border-border bg-sidebar p-2"
      style={{width: `${width}rem`}}
    >
      <div className="mb-2 flex items-center justify-between px-1">
        <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Sessions
        </span>
        <DropdownMenu>
          <DropdownMenuTrigger
            title="New session"
            aria-label="New session"
            className="flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
          >
            <Plus className="size-4"/>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className={"w-44"}>
            <DropdownMenuGroup>
              <DropdownMenuItem onClick={() => newSession(projectId, "claude")}>
                <Bot/>
                Claude Code
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => newSession(projectId, "shell")}>
                <Terminal/>
                Terminal
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={!git?.branch}
                onClick={() => setWorktreeOpen(true)}
              >
                <GitBranch/>
                Worktree
              </DropdownMenuItem>
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <div className="flex flex-1 flex-col gap-1.5 overflow-y-auto">
        {list.map((session) => (
          <SessionCard
            key={session.id}
            session={session}
            path={path}
            active={session.id === activeId}
            onSelect={() => activateSession(projectId, session.id)}
            onClose={() => requestClose(session)}
            onRename={(label) => renameSession(projectId, session.id, label)}
          />
        ))}
      </div>

      <WorktreeDialog
        open={worktreeOpen}
        onOpenChange={setWorktreeOpen}
        projectPath={path}
        currentBranch={git?.branch ?? ""}
        onCreate={createWorktree}
        onResume={resumeWorktree}
      />
      <CloseWorktreeDialog
        session={pendingClose}
        onCancel={() => setPendingClose(null)}
        onKeep={keepAndClose}
        onRemove={removeAndClose}
      />
      <ForceRemoveWorktreeDialog
        session={pendingForce}
        onCancel={() => setPendingForce(null)}
        onForceRemove={forceRemoveAndClose}
      />

      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize sidebar"
        onPointerDown={startDrag}
        onPointerMove={onDrag}
        onPointerUp={endDrag}
        className="absolute right-0 top-0 h-full w-1.5 cursor-col-resize touch-none transition-colors hover:bg-accent"
      />
    </aside>
  )
}
