import {Bell, Check} from "lucide-react"
import {useMatch, useNavigate} from "react-router-dom"

import {Button} from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {notificationsFrom} from "@/lib/notifications"
import {useProjects} from "@/lib/projects"
import type {SessionStatus} from "@/lib/session-events"
import {usePendingStatuses} from "@/lib/useSessionStatus"

function StatusIcon({status}: {status: SessionStatus}) {
  if (status === "waiting") {
    return <Bell className="size-4 shrink-0 text-amber-500"/>
  }
  return <Check className="size-4 shrink-0 text-emerald-500"/>
}

// NotificationsButton is the top-strip bell: the flat, cross-project queue of
// sessions needing attention (blocked on you, or a finished turn you have not
// seen), each routing to its own project on click — so a session blocked in a
// background project is reachable from wherever you are. The queue is in-page,
// so a full reload empties it until new events arrive, like the rest of the
// session state.
export function NotificationsButton() {
  const {projects, sessions, activateSession} = useProjects()
  const navigate = useNavigate()
  // The focused session (active project + its active session) is on screen, so
  // it is dropped from the queue — same match the provider derives it from.
  const activeProjectId = useMatch("/projects/:projectId/*")?.params.projectId
  const items = notificationsFrom(
    usePendingStatuses(),
    projects,
    sessions,
    activeProjectId,
  )

  const open = (projectId: string, sessionId: string) => {
    navigate(`/projects/${projectId}`)
    activateSession(projectId, sessionId)
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        title="Notifications"
        aria-label="Notifications"
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            className="relative shrink-0 text-muted-foreground"
          />
        }
      >
        <Bell className="size-4"/>
        {items.length > 0 && (
          <span
            aria-label={`${items.length} pending`}
            className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[0.625rem] font-medium leading-none text-primary-foreground"
          >
            {items.length}
          </span>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        {items.length === 0 ? (
          // Plain text, not DropdownMenuLabel: that is base-ui's Menu.GroupLabel
          // and throws outside a Menu.Group (it writes the label id into the
          // group context). The empty state is just a placeholder, not a label.
          <div className="px-2 py-1.5 text-xs text-muted-foreground">
            No notifications
          </div>
        ) : (
          items.map((item) => (
            <DropdownMenuItem
              key={item.id}
              className="gap-2"
              onClick={() => open(item.projectId, item.id)}
            >
              <StatusIcon status={item.status}/>
              <span className="flex min-w-0 flex-col">
                <span className="truncate">{item.sessionLabel}</span>
                <span className="truncate text-xs text-muted-foreground">
                  {item.projectName}
                </span>
              </span>
            </DropdownMenuItem>
          ))
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
