import { NavLink } from "react-router-dom"
import { Plus, Settings, Trash2 } from "lucide-react"
import type { ReactNode } from "react"
import { cn } from "@/lib/utils"
import { useProjects } from "@/lib/projects"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"

// RailLink is a Discord-style rail item: a rounded icon tile with a left "pill"
// indicator that grows when the route is active.
function RailLink({
  to,
  label,
  children,
}: {
  to: string
  label: string
  children: ReactNode
}) {
  return (
    <NavLink to={to} title={label} aria-label={label} className="group relative flex w-full justify-center">
      {({ isActive }) => (
        <>
          <span
            className={cn(
              "absolute left-0 top-1/2 w-1 -translate-y-1/2 rounded-r-full bg-foreground transition-all",
              isActive ? "h-8" : "h-0 group-hover:h-4",
            )}
          />
          <span
            className={cn(
              "flex size-11 items-center justify-center rounded-2xl bg-accent/40 text-muted-foreground transition-all hover:rounded-xl hover:bg-accent hover:text-accent-foreground",
              isActive && "rounded-xl bg-accent text-accent-foreground",
            )}
          >
            {children}
          </span>
        </>
      )}
    </NavLink>
  )
}

// RailButton is a rail item that fires an action instead of navigating.
function RailButton({
  label,
  onClick,
  children,
}: {
  label: string
  onClick: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={label}
      aria-label={label}
      className="flex size-11 items-center justify-center rounded-2xl bg-accent/40 text-muted-foreground transition-all hover:rounded-xl hover:bg-accent hover:text-accent-foreground"
    >
      {children}
    </button>
  )
}

// Rail is the Discord-style vertical navigation: home, the list of open
// projects, an "open project" button, and settings pinned to the bottom.
export function Rail() {
  const { projects, openProject, closeProject } = useProjects()

  return (
    <nav className="flex h-full w-[68px] flex-col items-center gap-2 border-r border-border bg-sidebar py-3">
      <div className="flex flex-1 flex-col items-center gap-2 overflow-y-auto">
        {projects.map((project) => (
          <ContextMenu key={project.id}>
            <ContextMenuTrigger className="flex w-full justify-center">
              <RailLink to={`/projects/${project.id}`} label={project.name}>
                <span className="text-sm font-semibold uppercase">
                  {project.name.slice(0, 2)}
                </span>
              </RailLink>
            </ContextMenuTrigger>
            <ContextMenuContent>
              <ContextMenuItem
                variant="destructive"
                onClick={() => closeProject(project.id)}
              >
                <Trash2 />
                Remove from list
              </ContextMenuItem>
            </ContextMenuContent>
          </ContextMenu>
        ))}

        <RailButton label="Open project" onClick={() => void openProject()}>
          <Plus className="size-5" />
        </RailButton>
      </div>

      <RailLink to="/settings" label="Settings">
        <Settings className="size-5" />
      </RailLink>
    </nav>
  )
}
