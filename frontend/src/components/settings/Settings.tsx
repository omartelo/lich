import { useState } from "react"
import type { ReactNode } from "react"
import { useParams } from "react-router-dom"
import { AppearanceSettings } from "./AppearanceSettings"
import { NotificationsSettings } from "./NotificationsSettings"
import { HotkeysSettings } from "./HotkeysSettings"
import { ProvidersSettings } from "./ProvidersSettings"
import { SandboxSettings } from "./SandboxSettings"
import { ProjectProvidersSettings } from "./ProjectProvidersSettings"
import { ProviderBinSettings } from "./ProviderBinSettings"
import { VersionControlSettings } from "./VersionControlSettings"
import { VcsToolsSetting } from "./VcsToolsSetting"
import { UpdatesSettings } from "./UpdatesSettings"
import { HelpSettings } from "./HelpSettings"
import { SearchInput } from "@/components/common/SearchInput"
import { enabledProviders, useProviders } from "@/lib/providers-store"
import { cn } from "@/lib/utils"

// A settings category: a nav entry plus the pane it renders. Global and project
// sections name their scope in the nav; enabled providers get their own sections,
// and footer sections sit at the bottom apart from configuration.
interface Section {
  id: string
  label: string
  accessibleLabel?: string
  group: "global" | "project" | "provider" | "footer"
  render: (projectId?: string) => ReactNode
}

// Base sections are always present. Their groups make scope visible before the
// user opens a pane: app-wide choices first, then the active project's.
const BASE_SECTIONS: Section[] = [
  {
    id: "appearance",
    label: "Appearance",
    group: "global",
    render: () => <AppearanceSettings />,
  },
  {
    id: "notifications",
    label: "Notifications",
    group: "global",
    render: () => <NotificationsSettings />,
  },
  { id: "hotkeys", label: "Hotkeys", group: "global", render: () => <HotkeysSettings /> },
  {
    id: "providers",
    label: "Providers",
    accessibleLabel: "Global Providers",
    group: "global",
    render: () => <ProvidersSettings />,
  },
  {
    // Global rather than project: most of what it says is about the machine —
    // whether there is a backend at all, what is in your ssh agent — and the
    // values are project-scoped the same way the provider sections' are.
    id: "sandbox",
    label: "Sandbox",
    group: "global",
    render: (id) => <SandboxSettings projectId={id} />,
  },
  {
    id: "project-providers",
    label: "Providers",
    accessibleLabel: "Project Providers",
    group: "project",
    render: (id) => <ProjectProvidersSettings projectId={id} />,
  },
  {
    id: "version-control",
    label: "Version Control",
    group: "project",
    // The tools are the machine's, not the project's, so they render above the
    // project block and outlive its "open a project first" state — a machine
    // with no git has to be able to read that with nothing open.
    render: (id) => (
      <>
        <VcsToolsSetting />
        <VersionControlSettings projectId={id} />
      </>
    ),
  },
]

// The two sections that configure nothing: they are about the app itself, and
// are reached once in a while rather than while setting a project up.
const FOOTER_SECTIONS: Section[] = [
  { id: "updates", label: "Updates", group: "footer", render: () => <UpdatesSettings /> },
  { id: "help", label: "Help", group: "footer", render: () => <HelpSettings /> },
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
    render: (id) => (
      <ProviderBinSettings
        providerId={provider.id}
        providerName={provider.name}
        providerBin={provider.binary}
        projectId={id}
      />
    ),
  }))
  const sections = [...BASE_SECTIONS, ...providerSections, ...FOOTER_SECTIONS]

  // Matched against the accessible label too: two sections are both labeled
  // "Providers", and their scope lives only in that longer name.
  const needle = query.toLowerCase()
  const filtered = sections.filter((section) =>
    `${section.label} ${section.accessibleLabel ?? ""}`.toLowerCase().includes(needle),
  )
  const globalSections = filtered.filter((section) => section.group === "global")
  const projectSections = filtered.filter((section) => section.group === "project")
  const provSections = filtered.filter((section) => section.group === "provider")
  const footerSections = filtered.filter((section) => section.group === "footer")
  // Fall back to the first section when the active one vanished (a provider was
  // disabled) or was filtered out of view.
  const current = sections.find((section) => section.id === active) ?? sections[0]

  const navButton = (section: Section) => (
    <button
      key={section.id}
      type="button"
      aria-label={section.accessibleLabel}
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
          <SearchInput
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search"
            aria-label="Search settings"
          />
        </div>
        {/* Scrolls on its own so a short window with several providers enabled
            shrinks this list instead of pushing the footer out of view. */}
        <nav className="flex min-h-0 flex-col gap-0.5 overflow-y-auto px-2 pb-3">
          {globalSections.length > 0 && (
            <div className="mb-1 px-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Global Settings
            </div>
          )}
          {globalSections.map(navButton)}
          {projectSections.length > 0 && (
            <div className="mt-4 mb-1 px-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Project Settings
            </div>
          )}
          {projectSections.map(navButton)}
          {provSections.length > 0 && (
            <div className="mt-4 mb-1 px-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Provider settings
            </div>
          )}
          {provSections.map(navButton)}
        </nav>

        {footerSections.length > 0 && (
          <nav className="mt-auto flex flex-col gap-0.5 border-t border-border px-2 py-2">
            {footerSections.map(navButton)}
          </nav>
        )}
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
