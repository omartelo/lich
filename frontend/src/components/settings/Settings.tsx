import { useState } from "react"
import type { ComponentType, ReactNode } from "react"
import { useParams } from "react-router-dom"
import {
  Bell,
  Blocks,
  CircleHelp,
  Download,
  GitBranch,
  Keyboard,
  Palette,
  ShieldCheck,
} from "lucide-react"
import { AppearanceSettings } from "./AppearanceSettings"
import { NotificationsSettings } from "./NotificationsSettings"
import { HotkeysSettings } from "./HotkeysSettings"
import { ProvidersPane } from "./ProvidersPane"
import { SandboxSettings } from "./SandboxSettings"
import { VersionControlSettings } from "./VersionControlSettings"
import { VcsToolsSetting } from "./VcsToolsSetting"
import { UpdatesSettings } from "./UpdatesSettings"
import { HelpSettings } from "./HelpSettings"
import { SearchInput } from "@/components/common/SearchInput"
import {
  DEFAULT_SECTION,
  readSettingsProvider,
  readSettingsQuery,
  readSettingsSection,
  writeSettingsProvider,
  writeSettingsQuery,
  writeSettingsSection,
} from "@/lib/settings-prefs"
import { useProviders } from "@/lib/providers-store"
import { cn } from "@/lib/utils"

// A settings category: a nav entry plus the pane it renders. The glyph is what
// makes the list scannable: every other list in the app is mark plus label,
// and this one was twelve identical strings.
interface Section {
  id: string
  label: string
  icon: ComponentType<{ className?: string }>
  /** Drawn with a hairline above it: what belongs to the open project rather
   * than to the machine starts here. Two rows do not need a heading each. */
  seam?: boolean
  /** Absent for the one pane the screen renders itself: Providers holds the
   * provider being looked at, and the nav needs that to name where you are. */
  render?: (projectId?: string) => ReactNode
}

// One list, six rows. Providers is a single section rather than one per enabled
// agent plus one for the project: the sidebar used to grow a line for every
// provider turned on, and the group that grew was the one nobody visits.
const SECTIONS: Section[] = [
  { id: "appearance", label: "Appearance", icon: Palette, render: () => <AppearanceSettings /> },
  {
    id: "notifications",
    label: "Notifications",
    icon: Bell,
    render: () => <NotificationsSettings />,
  },
  { id: "hotkeys", label: "Hotkeys", icon: Keyboard, render: () => <HotkeysSettings /> },
  { id: "providers", label: "Providers", icon: Blocks },
  {
    // Global rather than project: most of what it says is about the machine —
    // whether there is a backend at all, what is in your ssh agent — and the
    // values are project-scoped the same way the provider settings are.
    id: "sandbox",
    label: "Sandbox",
    icon: ShieldCheck,
    render: (id) => <SandboxSettings projectId={id} />,
  },
  {
    id: "version-control",
    label: "Version Control",
    icon: GitBranch,
    seam: true,
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
  { id: "updates", label: "Updates", icon: Download, render: () => <UpdatesSettings /> },
  { id: "help", label: "Help", icon: CircleHelp, render: () => <HelpSettings /> },
]

const ALL_SECTIONS = [...SECTIONS, ...FOOTER_SECTIONS]

// Settings is the per-project settings screen (not a modal): it fills the main
// area and sits on top of the persistent terminals, with the session sidebar
// kept beside it. The route carries the project id, which the provider and
// version control panes use for that project's overrides.
export function Settings() {
  const { projectId } = useParams()
  const providers = useProviders()
  // Seeded from the store and written through, so leaving the screen — a
  // session next door, another project — brings back the pane that was open
  // rather than the default one (settings-prefs).
  const [active, setActive] = useState(readSettingsSection)
  const [query, setQuery] = useState(readSettingsQuery)
  const [openProvider, setOpenProvider] = useState(readSettingsProvider)

  const openProviderScreen = (id: string) => {
    setOpenProvider(id)
    writeSettingsProvider(id)
  }

  const openSection = (id: string) => {
    setActive(id)
    writeSettingsSection(id)
    // Clicking Providers is the second way back out of a provider's own screen,
    // beside the one that screen offers: the nav entry has to mean the list it
    // names, or selecting it lands somewhere it does not describe.
    if (id === "providers") {
      openProviderScreen("")
    }
  }

  const search = (next: string) => {
    setQuery(next)
    writeSettingsQuery(next)
  }

  const needle = query.trim().toLowerCase()
  const matches = (section: Section) => section.label.toLowerCase().includes(needle)
  const sections = SECTIONS.filter(matches)
  const footerSections = FOOTER_SECTIONS.filter(matches)
  // A section id this build cannot place resolves to the default pane, not to
  // the first one: the ids a provider used to get its own section are still in
  // people's storage, and landing them on Appearance forever is the trap
  // docs/ceilings.md names.
  const current =
    ALL_SECTIONS.find((section) => section.id === active) ??
    ALL_SECTIONS.find((section) => section.id === DEFAULT_SECTION) ??
    ALL_SECTIONS[0]
  const providerName = providers.find((provider) => provider.id === openProvider)?.name ?? ""

  const navButton = (section: Section) => (
    <button
      key={section.id}
      type="button"
      onClick={() => openSection(section.id)}
      className={cn(
        "flex items-center gap-2 rounded-md px-3 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-accent/50 hover:text-accent-foreground",
        section.seam && "mt-2 border-t border-border pt-3",
        current.id === section.id && "bg-accent text-accent-foreground",
      )}
    >
      <section.icon className="size-4 shrink-0" />
      {section.label}
      {/* Where you are, when the pane goes a level deeper than the nav does. */}
      {section.id === "providers" && current.id === "providers" && providerName && (
        <span className="ml-auto truncate text-xs opacity-70">{providerName}</span>
      )}
    </button>
  )

  return (
    <div className="absolute inset-0 z-10 flex bg-background">
      <aside className="flex w-64 shrink-0 flex-col border-r border-border bg-sidebar">
        <div className="p-3">
          <SearchInput
            value={query}
            onChange={(event) => search(event.target.value)}
            placeholder="Search"
            aria-label="Search settings"
          />
        </div>
        {/* Scrolls on its own so a short window shrinks this list instead of
            pushing the footer out of view. */}
        <nav className="flex min-h-0 flex-col gap-0.5 overflow-y-auto px-2 pb-3">
          {sections.map(navButton)}
        </nav>

        {/* A search that matches nothing has to say so: an empty nav beside a
            pane that stayed put reads as a screen that broke. */}
        {sections.length === 0 && footerSections.length === 0 && (
          <p className="px-5 text-xs leading-relaxed text-muted-foreground">
            Nothing matches <span className="text-foreground">{query.trim()}</span>. Clear the
            search to see every section.
          </p>
        )}

        {footerSections.length > 0 && (
          <nav className="mt-auto flex flex-col gap-0.5 border-t border-border px-2 py-2">
            {footerSections.map(navButton)}
          </nav>
        )}
      </aside>

      <div className="flex-1 overflow-auto">
        <div className="w-full px-6 py-8 lg:px-10">
          {current.id === "providers" ? (
            <ProvidersPane
              projectId={projectId}
              openProvider={openProvider}
              onOpenProvider={openProviderScreen}
            />
          ) : (
            <>
              <h1 className="mb-4 text-2xl font-semibold text-foreground">{current.label}</h1>
              <div className="divide-y divide-border">{current.render?.(projectId)}</div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
