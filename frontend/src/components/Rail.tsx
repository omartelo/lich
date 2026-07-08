import { NavLink } from "react-router-dom"
import { Settings, SquareTerminal } from "lucide-react"
import type { ReactNode } from "react"
import { cn } from "@/lib/utils"
import { PROJECTS } from "@/components/TerminalHost"

interface RailButtonProps {
  to: string
  label: string
  children: ReactNode
}

function RailButton({ to, label, children }: RailButtonProps) {
  return (
    <NavLink
      to={to}
      title={label}
      aria-label={label}
      className={({ isActive }) =>
        cn(
          "flex size-10 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
          isActive && "bg-accent text-accent-foreground",
        )
      }
    >
      {children}
    </NavLink>
  )
}

// Rail is the vertical navigation bar. It will grow into a project rail; for now
// it holds a single project button plus the settings button at the bottom.
export function Rail() {
  return (
    <nav className="flex h-full w-14 flex-col items-center justify-between border-r border-border bg-sidebar py-3">
      <div className="flex flex-col gap-2">
        {PROJECTS.map((project) => (
          <RailButton
            key={project.id}
            to={`/projects/${project.id}`}
            label={project.label}
          >
            <SquareTerminal className="size-5" />
          </RailButton>
        ))}
      </div>
      <RailButton to="/settings" label="Settings">
        <Settings className="size-5" />
      </RailButton>
    </nav>
  )
}
