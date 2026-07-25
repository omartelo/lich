import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

// Notice is the quiet line a panel shows instead of content: loading, empty,
// or the reason it has nothing — "Not a git repository", "No open pull
// request". Muted text, never an icon or a border; the surface around it is
// what gives it weight.
//
// The default is the dock's density. A roomier surface passes its own padding
// and size rather than growing this into a variant scale.
export function Notice({ children, className }: { children: ReactNode; className?: string }) {
  return <p className={cn("px-3 py-4 text-xs text-muted-foreground", className)}>{children}</p>
}
