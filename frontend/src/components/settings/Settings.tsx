import { useState } from "react"
import type { ComponentType } from "react"
import { useParams } from "react-router-dom"
import { Search } from "lucide-react"
import { TerminalSettings } from "./TerminalSettings"
import { AppearanceSettings } from "./AppearanceSettings"
import { ClaudeCodeSettings } from "./ClaudeCodeSettings"
import { ProjectClaudeCodeSettings } from "./ProjectClaudeCodeSettings"
import { HotkeysSettings } from "./HotkeysSettings"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

// SECTIONS is the settings registry: adding a category is one new file under
// components/settings/ plus one entry here — the nav and the content pane both
// derive from this list. "app" sections apply globally; "project" sections hold
// per-project overrides and render under a "Project" group header in the nav.
const SECTIONS = [
  { id: "appearance", label: "Appearance", group: "app", Component: AppearanceSettings },
  { id: "terminal", label: "Terminal", group: "app", Component: TerminalSettings },
  { id: "hotkeys", label: "Hotkeys", group: "app", Component: HotkeysSettings },
  { id: "claude-code", label: "Claude Code", group: "app", Component: ClaudeCodeSettings },
  {
    id: "project-claude-code",
    label: "Claude Code",
    group: "project",
    Component: ProjectClaudeCodeSettings,
  },
] as const satisfies ReadonlyArray<{
  id: string
  label: string
  group: "app" | "project"
  Component: ComponentType<{ projectId?: string }>
}>

type SectionId = (typeof SECTIONS)[number]["id"]

// Settings is the per-project settings screen (not a modal): it fills the main
// area and sits on top of the persistent terminals, which stay mounted and
// running behind it, with the session sidebar kept beside it. The route carries
// the project id, which the "project" sections use for that project's
// overrides. A category nav sits on the left; content is on the right.
export function Settings() {
  const { projectId } = useParams()
  const [active, setActive] = useState<SectionId>("terminal")
  const [query, setQuery] = useState("")

  const filtered = SECTIONS.filter((section) =>
    section.label.toLowerCase().includes(query.toLowerCase()),
  )
  const appSections = filtered.filter((section) => section.group === "app")
  const projectSections = filtered.filter((section) => section.group === "project")
  const activeSection = SECTIONS.find((section) => section.id === active)
  const ActiveComponent = activeSection?.Component

  const navButton = (section: (typeof SECTIONS)[number]) => (
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
  )

  return (
    <div className="absolute inset-0 z-10 flex bg-background">
      <aside className="flex w-64 shrink-0 flex-col border-r border-border bg-sidebar">
        <div className="p-3">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search"
              aria-label="Search settings"
              className="pl-8"
            />
          </div>
        </div>
        <nav className="flex flex-col gap-0.5 px-2 pb-3">
          {appSections.map(navButton)}
          {projectSections.length > 0 && (
            <div className="mt-4 mb-1 px-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Project
            </div>
          )}
          {projectSections.map(navButton)}
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
                <ActiveComponent projectId={projectId} />
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
