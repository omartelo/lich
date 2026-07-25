import type { LucideIcon } from "lucide-react"
import type { ReactNode } from "react"

interface EmptyScreenProps {
  icon: LucideIcon
  title: string
  description: string
  /** The single way forward — one primary Button, nothing else. */
  children: ReactNode
}

// EmptyScreen is the full-area landing state that covers the terminal host when
// there is nothing to show: no project open, no session open. Centred glyph,
// title, one line of copy, one action. It owns the overlay itself (absolute +
// z-10 + bg-background) because that is what puts it over the terminals.
export function EmptyScreen({ icon: Icon, title, description, children }: EmptyScreenProps) {
  return (
    <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-4 bg-background">
      <Icon className="size-12 text-muted-foreground" />
      <div className="text-center">
        <h1 className="text-lg font-semibold text-foreground">{title}</h1>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>
      {children}
    </div>
  )
}
