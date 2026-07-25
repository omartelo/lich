import { Search } from "lucide-react"
import type { ComponentProps } from "react"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

// A text field with the magnifier tucked inside its left edge. Everything else
// is a plain Input — the caller keeps ownership of value, keys and placeholder.
export function SearchInput({ className, ...props }: ComponentProps<typeof Input>) {
  return (
    <div className="relative">
      <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input {...props} className={cn("pl-8", className)} />
    </div>
  )
}
