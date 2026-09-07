import { useEffect, useMemo, useState } from "react"
import type { ComponentType, KeyboardEvent, ReactNode } from "react"
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
import { enabledProviders, useProviders } from "@/lib/providers-store"
import {
  expandEntries,
  hitPath,
  searchSettings,
  sectionLabel,
  type SettingHit,
} from "@/lib/settings-index"
import { HighlightProvider, type HighlightTarget } from "./setting-highlight"
import { cn } from "@/lib/utils"

// A settings category: a nav entry plus the pane it renders. The glyph is what
// makes the list scannable: every other list in the app is mark plus label,
// and this one was twelve identical strings.
interface Section {
  /** Matches an entry in SETTING_SECTIONS, which is where the label lives: the
   * search names sections too, and one of the two would have gone stale. */
  id: string
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
  { id: "appearance", icon: Palette, render: () => <AppearanceSettings /> },
  {
    id: "notifications",
    icon: Bell,
    render: () => <NotificationsSettings />,
  },
  { id: "hotkeys", icon: Keyboard, render: () => <HotkeysSettings /> },
  { id: "providers", icon: Blocks },
  {
    // Global rather than project: most of what it says is about the machine —
    // whether there is a backend at all, what is in your ssh agent — and the
    // values are project-scoped the same way the provider settings are.
    id: "sandbox",
    icon: ShieldCheck,
    render: (id) => <SandboxSettings projectId={id} />,
  },
  {
    id: "version-control",
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
  { id: "updates", icon: Download, render: () => <UpdatesSettings /> },
  { id: "help", icon: CircleHelp, render: () => <HelpSettings /> },
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
  // The result the arrow keys are on, and the control a chosen result lit up.
  const [cursor, setCursor] = useState(0)
  const [lit, setLit] = useState<HighlightTarget>({ title: "", group: "" })

  const hits = useMemo(
    () => searchSettings(query, expandEntries(enabledProviders(providers))),
    [query, providers],
  )

  // The highlight is a pointer, not a state to live in: it says "you were just
  // sent here", and a fill that stayed would read as a selection nobody made.
  useEffect(() => {
    if (lit.title === "") {
      return
    }
    const timer = setTimeout(() => setLit({ title: "", group: "" }), 2000)
    return () => clearTimeout(timer)
  }, [lit])

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
    setCursor(0)
  }

  // Open the pane the result lives in, walk into the provider when it has one,
  // and tell the pane which control to light up. The box keeps its text: a
  // search is often two or three results deep before it is the right one.
  const choose = (hit: SettingHit) => {
    setActive(hit.section)
    writeSettingsSection(hit.section)
    openProviderScreen(hit.providerId ?? "")
    // A per-provider block carries the provider's name as its group, and that
    // is not a group inside the pane: walking into that provider's screen is
    // what already told the two apart.
    setLit({ title: hit.title, group: hit.providerId ? "" : (hit.group ?? "") })
  }

  const onSearchKey = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Escape") {
      search("")
      return
    }
    if (hits.length === 0) {
      return
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault()
      const step = event.key === "ArrowDown" ? 1 : hits.length - 1
      setCursor((at) => (at + step) % hits.length)
      return
    }
    if (event.key === "Enter") {
      event.preventDefault()
      const hit = hits[cursor]
      if (hit) {
        choose(hit)
      }
    }
  }

  const searching = query.trim() !== ""
  const sections = SECTIONS
  const footerSections = FOOTER_SECTIONS
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
      {sectionLabel(section.id)}
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
            onKeyDown={onSearchKey}
            placeholder="Search settings"
            aria-label="Search settings"
          />
        </div>

        {/* While there is something typed, the nav is the results: the sections
            are eight labels, and what people look for are the controls under
            them. Esc puts the nav back. */}
        {searching ? (
          <>
            {hits.length > 0 && (
              <p className="px-5 pb-1.5 font-mono text-[0.65625rem] uppercase tracking-wider text-muted-foreground">
                {hits.length} {hits.length === 1 ? "setting" : "settings"}
              </p>
            )}
            <nav className="flex min-h-0 flex-col gap-0.5 overflow-y-auto px-2 pb-3">
              {hits.map((hit, index) => (
                <button
                  key={`${hit.section}-${hit.providerId ?? ""}-${hit.group ?? ""}-${hit.title}`}
                  type="button"
                  onClick={() => choose(hit)}
                  onMouseEnter={() => setCursor(index)}
                  className={cn(
                    "flex flex-col rounded-md px-3 py-1.5 text-left transition-colors hover:bg-accent/50",
                    index === cursor && "bg-accent",
                  )}
                >
                  {/* A section is its own result, and "Updates / Updates" would
                      be the path saying the name twice. */}
                  {hitPath(hit, sectionLabel(hit.section)) !== hit.title && (
                    <span
                      className={cn(
                        "font-mono text-[0.65625rem] tracking-wide text-muted-foreground",
                        index === cursor && "text-accent-foreground/75",
                      )}
                    >
                      {hitPath(hit, sectionLabel(hit.section))}
                    </span>
                  )}
                  <span
                    className={cn(
                      "text-sm leading-snug text-foreground",
                      index === cursor && "text-accent-foreground",
                    )}
                  >
                    {hit.title}
                  </span>
                </button>
              ))}
            </nav>
            {hits.length === 0 && (
              <p className="px-5 text-xs leading-relaxed text-muted-foreground">
                Nothing matches <span className="text-foreground">{query.trim()}</span>. The search
                reads the name of every setting, not their values.
              </p>
            )}
          </>
        ) : (
          /* Scrolls on its own so a short window shrinks this list instead of
             pushing the footer out of view. */
          <nav className="flex min-h-0 flex-col gap-0.5 overflow-y-auto px-2 pb-3">
            {sections.map(navButton)}
          </nav>
        )}

        <nav className="mt-auto flex flex-col gap-0.5 border-t border-border px-2 py-2">
          {footerSections.map(navButton)}
        </nav>
      </aside>

      <div className="flex-1 overflow-auto">
        <div className="w-full px-6 py-8 lg:px-10">
          <HighlightProvider value={lit}>
            {current.id === "providers" ? (
              <ProvidersPane
                projectId={projectId}
                openProvider={openProvider}
                onOpenProvider={openProviderScreen}
              />
            ) : (
              <>
                <h1 className="mb-4 text-2xl font-semibold text-foreground">
                  {sectionLabel(current.id)}
                </h1>
                <div className="divide-y divide-border">{current.render?.(projectId)}</div>
              </>
            )}
          </HighlightProvider>
        </div>
      </div>
    </div>
  )
}
