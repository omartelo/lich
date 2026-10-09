import { useNavigate } from "react-router-dom"
import { Bell, Check, LoaderCircle } from "lucide-react"
import { useSortable } from "@dnd-kit/sortable"
import { CloseButton } from "@/components/common/CloseButton"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { dragStyle } from "@/lib/use-sortable-list"
import { cn } from "@/lib/utils"
import { useProjectStatus } from "@/lib/session/use-session-status"
import type { Project } from "@/lib/api-types"

interface ProjectTabProps {
  project: Project
  sessionIds: readonly string[]
  // Where the tab leads: the screen this project was last showing, which is not
  // always its terminals (project-route).
  to: string
  active: boolean
  onClose: () => void
}

// The tab is its own drag grip for reordering the strip — no separate handle.
export function ProjectTab({ project, sessionIds, to, active, onClose }: ProjectTabProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: project.id,
  })
  // What the project's sessions are up to while you are looking elsewhere. The
  // active tab never badges: its cards are already on screen saying the same
  // thing, in more detail and per session.
  const status = useProjectStatus(sessionIds)
  const badge = active ? null : status
  const navigate = useNavigate()

  return (
    <div
      ref={setNodeRef}
      style={dragStyle(transform, transition)}
      className={cn("shrink-0", isDragging && "z-10 rounded-md bg-accent shadow-md")}
      {...attributes}
      {...listeners}
    >
      <Tooltip>
        <TooltipTrigger
          render={
            <button
              type="button"
              onClick={() => navigate(to)}
              className={cn(
                "group flex h-8 max-w-52 items-center gap-2 rounded-md px-3 text-sm text-muted-foreground transition-colors hover:text-foreground",
                active && "bg-sidebar font-medium text-foreground",
              )}
            />
          }
        >
          {(badge === "busy" || badge === "compacting") && (
            <LoaderCircle className="size-3 shrink-0 animate-spin" />
          )}
          {badge === "done" && <Check className="size-3 shrink-0 text-tone-pass" />}
          {badge === "waiting" && <Bell className="size-3 shrink-0 text-tone-wait" />}
          <span className="truncate">{project.name}</span>
          <CloseButton
            label={`Close ${project.name}`}
            onClick={(event) => {
              event.stopPropagation()
              onClose()
            }}
          />
        </TooltipTrigger>
        <TooltipContent
          side="bottom"
          className="max-w-xs border border-border bg-card text-foreground"
        >
          <div className="flex flex-col gap-1.5">
            <span className="font-medium">{project.name}</span>
            <span className="break-all font-mono text-muted-foreground">{project.path}</span>
          </div>
        </TooltipContent>
      </Tooltip>
    </div>
  )
}
