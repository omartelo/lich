import { useState } from "react"
import type { ReactNode } from "react"
import { useParams } from "react-router-dom"
import { Search } from "lucide-react"
import { TerminalSettings } from "./TerminalSettings"
import { AppearanceSettings } from "./AppearanceSettings"
import { HotkeysSettings } from "./HotkeysSettings"
import { ProvidersSettings } from "./ProvidersSettings"
import { ProviderBinSettings } from "./ProviderBinSettings"
import { enabledProviders, useProviders } from "@/lib/providers-store"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

// A settings category: a nav entry plus the pane it renders. "app" sections are
// global; "provider" sections are the per-provider config that appears when a
// provider is enabled, and render under a "Provider settings" nav header.
interface Section {
  id: string
  label: string
  group: "app" | "provider"
  render: (projectId?: string) => ReactNode
}

// Base sections are always present. The "Providers" hub holds the enable
// toggles; enabling a provider adds its own section below (see providerSections).
const BASE_SECTIONS: Section[] = [
  { id: "appearance", label: "Appearance", group: "app", render: () => <AppearanceSettings /> },
  { id: "terminal", label: "Terminal", group: "app", render: () => <TerminalSettings /> },
  { id: "hotkeys", label: "Hotkeys", group: "app", render: () => <HotkeysSettings /> },
  { id: "providers", label: "Providers", group: "app", render: () => <ProvidersSettings /> },
]

// Settings is the per-project settings screen (not a modal): it fills the main
// area and sits on top of the persistent terminals, with the session sidebar
// kept beside it. The route carries the project id, which the provider sections
// use for that project's overrides.
export function Settings() {
  const { projectId } = useParams()
  const providers = useProviders()
  const [active, setActive] = useState("providers")
  const [query, setQuery] = useState("")

  const providerSections: Section[] = enabledProviders(providers).map((provider) => ({
    id: `provider-${provider.id}`,
    label: provider.name,
    group: "provider",
    render: (id) => <ProviderBinSettings providerId={provider.id} projectId={id} />,
  }))
  const sections = [...BASE_SECTIONS, ...providerSections]

  const filtered = sections.filter((section) =>
    section.label.toLowerCase().includes(query.toLowerCase()),
  )
  const appSections = filtered.filter((section) => section.group === "app")
  const provSections = filtered.filter((section) => section.group === "provider")
  // Fall back to the first section when the active one vanished (a provider was
  // disabled) or was filtered out of view.
  const current = sections.find((section) => section.id === active) ?? sections[0]

  const navButton = (section: Section) => (
    <button
      key={section.id}
      type="button"
      onClick={() => setActive(section.id)}
      className={cn(
        "rounded-md px-3 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
        current.id === section.id && "bg-accent text-accent-foreground",
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
          {provSections.length > 0 && (
            <div className="mt-4 mb-1 px-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Provider settings
            </div>
          )}
          {provSections.map(navButton)}
        </nav>
      </aside>

      <div className="flex-1 overflow-auto">
        <div className="mx-auto max-w-3xl px-8 py-8">
          <h1 className="mb-4 text-2xl font-semibold text-foreground">{current.label}</h1>
          <div className="divide-y divide-border">{current.render(projectId)}</div>
        </div>
      </div>
    </div>
  )
}
