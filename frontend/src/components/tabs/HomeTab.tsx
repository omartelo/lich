import { Link } from "react-router-dom"
import { Home } from "lucide-react"
import { cn } from "@/lib/utils"

interface HomeTabProps {
  // The screen Home was last showing (project-route), like every other tab.
  to: string
  active: boolean
}

// HomeTab is the pinned, non-closable first tab: a plain shell at the system
// home directory. Icon-only and rendered outside the project reorder list, so
// it is never draggable and never closable.
export function HomeTab({ to, active }: HomeTabProps) {
  return (
    <Link
      to={to}
      title="Home"
      aria-label="Home"
      className={cn(
        "flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground",
        active && "bg-accent text-accent-foreground",
      )}
    >
      <Home className="size-4" />
    </Link>
  )
}
