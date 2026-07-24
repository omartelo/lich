import { NavLink } from "react-router-dom"
import { Home } from "lucide-react"
import { cn } from "@/lib/utils"

interface HomeTabProps {
  projectId: string
}

// HomeTab is the pinned, non-closable first tab: a plain shell at the system
// home directory. Icon-only and rendered outside the project reorder list, so
// it is never draggable and never closable.
export function HomeTab({ projectId }: HomeTabProps) {
  return (
    <NavLink
      to={`/projects/${projectId}`}
      title="Home"
      aria-label="Home"
      className={({ isActive }) =>
        cn(
          "flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground",
          isActive && "bg-accent text-accent-foreground",
        )
      }
    >
      <Home className="size-4" />
    </NavLink>
  )
}
