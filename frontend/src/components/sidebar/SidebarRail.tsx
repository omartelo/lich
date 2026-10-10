import { useEffect } from "react"
import { useMatch, useNavigate } from "react-router-dom"
import { PanelLeft, Plus } from "lucide-react"
import { cn } from "@/lib/utils"
import { useT } from "@/lib/i18n/i18n"
import { SIDEBAR_MORPH } from "@/lib/view-transition"
import { useProjects } from "@/providers/projects"
import {
  activeSessionId,
  sessionsOf,
  type Session,
  workspaceSessions,
} from "@/lib/session/sessions"
import { sidebarCards, sidebarGroups } from "@/lib/session/sidebar-groups"
import { registerSidebarCards } from "@/lib/session/sidebar-cards-store"
import { resolveGroups } from "@/lib/session/panes"
import { useStoredGroups } from "@/lib/session/panes-store"
import { useSessionAgent } from "@/lib/session/use-session-agent"
import { useSessionStatus, useSessionUnread } from "@/lib/session/use-session-status"
import { SessionStatusIcon } from "./SessionStatusIcon"
import { SessionTooltip } from "./SessionTooltip"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

interface RailSessionProps {
  session: Session
  // The project's own directory, the fallback for a session with no path of its
  // own and no cwd reported yet.
  projectPath: string
  // The project this session sits in, the scope its sandbox rung is read in.
  projectId: string
  active: boolean
  onSelect: () => void
}

// One session, reduced to the glyph it already wears on its card: the provider
// mark inside the status ring. The ring is the whole reason the rail exists —
// collapsing the sidebar must not cost the readout you watch all day — so it is
// drawn from the same component the card uses, not a second implementation.
// Same for the tooltip: at this width it is the only place the card's words can
// go, so it is the card's own tooltip, not a shortened one.
function RailSession({ session, projectPath, projectId, active, onSelect }: RailSessionProps) {
  const status = useSessionStatus(session.id)
  const unread = useSessionUnread(session.id)
  const agent = useSessionAgent(session.id)
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            onClick={onSelect}
            aria-label={session.label}
            className={cn(
              "flex size-8 shrink-0 items-center justify-center rounded-md transition-colors hover:bg-accent/60",
              active && "bg-accent text-accent-foreground",
            )}
          />
        }
      >
        <SessionStatusIcon
          kind={agent ?? session.kind}
          status={status}
          unread={unread}
          morphName={SIDEBAR_MORPH.session(session.id)}
        />
      </TooltipTrigger>
      <SessionTooltip session={session} path={projectPath} projectId={projectId} />
    </Tooltip>
  )
}

interface SidebarRailProps {
  onExpand: () => void
}

// SidebarRail is the collapsed session sidebar: the same list, the same order,
// the same worktree grouping — one hairline where the open sidebar draws a
// titled divider, since a checkout's name does not fit in 3rem.
//
// It selects and it creates, and that is all: no close, no rename, no context
// menu, no drag. Every one of those aims at a 32px target for an action the
// open sidebar already does better, so the rail sends you back to it instead of
// growing a second, poorer copy of the card.
export function SidebarRail({ onExpand }: SidebarRailProps) {
  const t = useT()
  const { projects, sessions, newSession, activateSession } = useProjects()
  const match = useMatch("/projects/:projectId/*")
  const projectId = match?.params.projectId
  const navigate = useNavigate()
  // Subscribed rather than read: a pane mutation that touches no session state —
  // the add shortcut, a pane's ×, a drag swap — notifies this store and nothing
  // else, so a rail holding no listener would draw yesterday's blocks until some
  // unrelated render came along. Resolved ahead of the no-project bail below:
  // hooks cannot sit behind it.
  const stored = useStoredGroups()
  // Same order as the expanded sidebar, split's block and all: the rail is
  // that list with the words taken out. Reconciled the same way too — the
  // stored value is not the truth on its own, and a rail drawing a wall the
  // open sidebar has already dropped is the same list disagreeing with itself.
  // Against every project, as the sidebar does: a wall of one card here and one
  // in another project is still a wall. Its guests are not drawn at this width.
  const list = sessionsOf(sessions, projectId ?? "")
  const groups = sidebarGroups(list, resolveGroups(stored, workspaceSessions(sessions)))
  // The rail draws every card, folded blocks included, so the shortcuts walk
  // them all while it is the sidebar on screen.
  useEffect(() => registerSidebarCards(() => sidebarCards(groups, () => false)))

  if (!projectId) {
    return null
  }

  const path = projects.find((p) => p.id === projectId)?.path ?? ""
  // Unlike the open sidebar, a full-screen route (Settings, Pulls) does not put
  // the highlight out: those screens have no card of their own here to carry
  // it, so dropping it would leave the rail with nothing lit at all.
  const activeId = activeSessionId(sessions, projectId)

  const select = (id: string) => {
    activateSession(projectId, id)
    navigate(`/projects/${projectId}`)
  }

  return (
    <aside
      className="flex w-8 shrink-0 flex-col items-center gap-1 pb-1.5"
      style={{ viewTransitionName: SIDEBAR_MORPH.panel }}
    >
      <Tooltip>
        <TooltipTrigger
          render={
            <button
              type="button"
              onClick={onExpand}
              aria-label={t("sidebar.sidebarRail.expand")}
              className="flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent/60 hover:text-foreground"
            />
          }
        >
          <PanelLeft className="size-4" style={{ viewTransitionName: SIDEBAR_MORPH.toggle }} />
        </TooltipTrigger>
        <TooltipContent side="right">{t("sidebar.sidebarRail.expand")}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger
          render={
            <button
              type="button"
              onClick={() => newSession(projectId)}
              aria-label={t("sidebar.sidebarRail.newSession")}
              className="flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent/60 hover:text-foreground"
            />
          }
        >
          <Plus className="size-4" style={{ viewTransitionName: SIDEBAR_MORPH.newSession }} />
        </TooltipTrigger>
        {/* No provider menu at this width: the plus spawns the default provider,
            the same session the New session shortcut makes. */}
        <TooltipContent side="right">{t("sidebar.sidebarRail.newSession")}</TooltipContent>
      </Tooltip>
      <div className="flex w-full flex-1 flex-col items-center gap-1 overflow-y-auto overflow-x-hidden">
        {groups.map((group, index) => (
          <div key={group.key} className="flex w-full flex-col items-center gap-1">
            {index > 0 && <span className="my-1 h-px w-5 shrink-0 bg-border" />}
            {group.sessions.map((session) => (
              <RailSession
                key={session.id}
                session={session}
                projectPath={path}
                projectId={projectId}
                active={session.id === activeId}
                onSelect={() => select(session.id)}
              />
            ))}
          </div>
        ))}
      </div>
    </aside>
  )
}
