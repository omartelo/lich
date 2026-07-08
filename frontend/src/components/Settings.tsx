import { useState } from "react"
import type { ComponentType } from "react"
import { Search } from "lucide-react"
import { TerminalSettings } from "@/components/settings/TerminalSettings"
import { AppearanceSettings } from "@/components/settings/AppearanceSettings"
import { cn } from "@/lib/utils"

// SECTIONS is the settings registry: adding a category is one new file under
// components/settings/ plus one entry here — the nav and the content pane both
// derive from this list.
const SECTIONS = [
  { id: "appearance", label: "Appearance", Component: AppearanceSettings },
  { id: "terminal", label: "Terminal", Component: TerminalSettings },
] as const satisfies ReadonlyArray<{
  id: string
  label: string
  Component: ComponentType
}>

type SectionId = (typeof SECTIONS)[number]["id"]

// Settings is a full screen (not a modal): it fills the main area and sits on
// top of the persistent terminals, which stay mounted and running behind it. A
// category nav on the left mirrors the terminal Rail; content is on the right.
export function Settings() {
  const [active, setActive] = useState<SectionId>("terminal")
  const [query, setQuery] = useState("")

  const filtered = SECTIONS.filter((section) =>
    section.label.toLowerCase().includes(query.toLowerCase()),
  )
  const activeSection = SECTIONS.find((section) => section.id === active)
  const ActiveComponent = activeSection?.Component

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
          {filtered.map((section) => (
            <button
              key={section.id}
              type="button"
              onClick={() => setActive(section.id)}
              className={cn(
                "rounded-md px-3 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
                active === section.id && "bg-accent text-accent-foreground",
              )}
            >
              {section.label}
            </button>
          ))}
        </nav>
      </aside>

      <div className="flex-1 overflow-auto">
        <div className="mx-auto max-w-3xl px-8 py-8">
          {activeSection && ActiveComponent && (
            <>
              <h1 className="mb-4 text-2xl font-semibold text-foreground">
                {activeSection.label}
              </h1>
              <div className="divide-y divide-border">
                <ActiveComponent />
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
