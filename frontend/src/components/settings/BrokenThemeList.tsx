import { TriangleAlert } from "lucide-react"
import type { BrokenTheme } from "@/lib/api-types"
import { Button } from "@/components/ui/button"

interface BrokenThemeListProps {
  themes: readonly BrokenTheme[]
  onRemove: (theme: BrokenTheme) => void
}

// A theme lich cannot load has no colors to preview, so it is a row under the
// strip rather than a card in it, and the reason is text, not a tooltip: the
// reason is the whole point, and a tooltip is out of reach from a keyboard.
export function BrokenThemeList({ themes, onRemove }: BrokenThemeListProps) {
  if (themes.length === 0) return null
  return (
    <section aria-label="Themes that can't load" className="mb-3 border-t border-border pt-2">
      <h3 className="flex items-center gap-1.5 pb-1 text-2xs font-medium uppercase tracking-wide text-muted-foreground">
        <TriangleAlert className="size-3.5" />
        {themes.length === 1 ? "1 theme can't load" : `${themes.length} themes can't load`}
      </h3>
      <ul>
        {themes.map((theme) => (
          <li
            key={theme.id}
            className="-mx-2 grid grid-cols-[minmax(6rem,auto)_1fr_auto] items-center gap-x-4 rounded-md px-2 py-1 hover:bg-accent/50"
          >
            <span className="font-mono text-xs text-foreground">{theme.id}</span>
            <span className="text-xs break-words text-muted-foreground">{theme.reason}</span>
            <Button
              type="button"
              variant="ghost"
              size="xs"
              className="text-destructive hover:text-destructive"
              aria-label={`Remove ${theme.id}`}
              onClick={() => onRemove(theme)}
            >
              Remove
            </Button>
          </li>
        ))}
      </ul>
    </section>
  )
}
