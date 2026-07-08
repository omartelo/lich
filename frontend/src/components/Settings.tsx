import { useEffect, useMemo, useState } from "react"
import type { ReactNode } from "react"
import { Search } from "lucide-react"
import { Service as FontService } from "../../bindings/github.com/skipodotdev/skipo/internals/fonts"
import { DEFAULT_FONT, useSettings } from "@/lib/settings"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { cn } from "@/lib/utils"

const CATEGORIES = [{ id: "terminal", label: "Terminal" }] as const
type CategoryId = (typeof CATEGORIES)[number]["id"]

// SettingRow is a single setting: label (and optional description) on the left,
// the control on the right, separated from the next row by a hairline divider.
function SettingRow({
  label,
  description,
  children,
}: {
  label: string
  description?: string
  children: ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-6 border-b border-border py-4">
      <div className="flex flex-col gap-0.5">
        <span className="text-sm text-foreground">{label}</span>
        {description && (
          <span className="text-xs text-muted-foreground">{description}</span>
        )}
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  )
}

function TerminalSettings() {
  const { font, setFont } = useSettings()
  const [families, setFamilies] = useState<string[]>([])

  useEffect(() => {
    void FontService.List()
      .then((list) => setFamilies(list ?? []))
      .catch(() => setFamilies([]))
  }, [])

  // Always offer the bundled default and the current selection, even if
  // fontconfig does not list them (the bundled font is not OS-installed).
  const options = useMemo(
    () => Array.from(new Set([DEFAULT_FONT, font, ...families])),
    [families, font],
  )

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold text-foreground">Terminal</h1>
      <div className="border-t border-border">
        <SettingRow
          label="Font"
          description="Font family used to render the terminal."
        >
          <Select value={font} onValueChange={(value) => value && setFont(value)}>
            <SelectTrigger className="w-64">
              <SelectValue placeholder="Select a font" />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {options.map((family) => (
                  <SelectItem key={family} value={family}>
                    {family}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </SettingRow>
      </div>
    </div>
  )
}

// Settings is a full screen (not a modal): it fills the main area and sits on
// top of the persistent terminals, which stay mounted and running behind it. A
// category nav on the left mirrors the terminal Rail; content is on the right.
export function Settings() {
  const [active, setActive] = useState<CategoryId>("terminal")
  const [query, setQuery] = useState("")

  const filtered = CATEGORIES.filter((category) =>
    category.label.toLowerCase().includes(query.toLowerCase()),
  )

  return (
    <div className="absolute inset-0 z-10 flex bg-background">
      <aside className="flex w-64 shrink-0 flex-col border-r border-border bg-sidebar">
        <div className="p-3">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search"
              aria-label="Search settings"
              className="h-9 w-full rounded-md border border-border bg-background pl-8 pr-3 text-sm text-foreground outline-none placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring"
            />
          </div>
        </div>
        <nav className="flex flex-col gap-0.5 px-2 pb-3">
          {filtered.map((category) => (
            <button
              key={category.id}
              type="button"
              onClick={() => setActive(category.id)}
              className={cn(
                "rounded-md px-3 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                active === category.id && "bg-accent text-accent-foreground",
              )}
            >
              {category.label}
            </button>
          ))}
        </nav>
      </aside>

      <div className="flex-1 overflow-auto">
        <div className="mx-auto max-w-3xl px-8 py-8">
          {active === "terminal" && <TerminalSettings />}
        </div>
      </div>
    </div>
  )
}
