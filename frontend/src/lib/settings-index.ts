// What the settings search looks through. The nav has eight labels; the things
// people actually look for are the two dozen controls under them, and until
// this existed a search for "theme" or "ssh" answered that nothing matched.
//
// The index is written out rather than derived from the rendered tree: the
// panes are React components whose blocks only exist once rendered, and the
// suite runs in node. settings-index.test.ts is what keeps the two in step, by
// reading the same literals out of the source and failing on a block that
// nobody added here.
import { HOTKEY_ACTIONS, HOTKEY_GROUPS } from "@/lib/hotkeys"

export interface SettingEntry {
  /** The nav section that opens it, by the id Settings.tsx gives it. */
  section: string
  /** The group inside the pane, where the pane has groups. Part of the path a
   * result shows. */
  group?: string
  /** The block's own title, verbatim from its SettingBlock. */
  title: string
  /** Words someone would type that the title does not contain. The reason a
   * search for "cost" finds a block called "Spend ceiling". */
  also?: string
  /** Lives on a provider's own screen, so it exists once per enabled provider
   * and reaching it is two steps rather than one. */
  perProvider?: boolean
}

// Every SettingBlock in the app, in the order its pane renders it.
export const SETTING_ENTRIES: readonly SettingEntry[] = [
  {
    section: "appearance",
    group: "Interface",
    title: "Theme",
    also: "colors dark light terminal palette",
  },
  { section: "appearance", group: "Interface", title: "Zoom" },
  { section: "appearance", group: "Terminal", title: "Text size" },
  { section: "appearance", group: "Terminal", title: "Font", also: "typeface monospace" },
  { section: "appearance", group: "Footer", title: "Footer layout", also: "status bar arrange" },
  { section: "appearance", group: "Footer", title: "Spend ceiling", also: "cost budget usd" },
  { section: "notifications", title: "Notify me when a session needs input" },
  { section: "notifications", title: "Notify me when a session finishes working" },
  { section: "providers", title: "Default provider", also: "agent new session" },
  {
    section: "providers",
    title: "Plan usage",
    also: "quota subscription limit",
    perProvider: true,
  },
  { section: "providers", title: "Binary", also: "path executable command", perProvider: true },
  {
    section: "providers",
    title: "Skip permission prompts",
    also: "yolo dangerous approvals",
    perProvider: true,
  },
  { section: "providers", title: "Right now", also: "open sessions docs", perProvider: true },
  { section: "sandbox", title: "Which sessions run confined", also: "isolate bwrap seatbelt" },
  { section: "sandbox", title: "What a confined session may carry in" },
  { section: "sandbox", title: "SSH agent", also: "push keys" },
  { section: "sandbox", title: "GitHub token", also: "gh credentials" },
  { section: "version-control", title: "Command-line tools", also: "git gh install" },
  { section: "version-control", title: "GitHub account", also: "login gh switch" },
  { section: "updates", title: "Application", also: "version upgrade" },
  { section: "updates", title: "What's new", also: "changelog release notes" },
  { section: "updates", title: "lich plugin", also: "hooks install" },
  { section: "help", title: "Report a bug", also: "issue github" },
  { section: "help", title: "Log file", also: "debug logs" },
  { section: "help", title: "About", also: "version license" },
]

// The keyboard shortcuts are not blocks, so they are not written out twice:
// HOTKEY_ACTIONS already carries a label per action, and it is the list the
// pane itself renders. A shortcut added to the app is searchable the day it
// lands, with no entry to forget.
export function hotkeyEntries(): SettingEntry[] {
  return HOTKEY_ACTIONS.map((action) => ({
    section: "hotkeys",
    group: HOTKEY_GROUPS.find((group) => group.id === action.group)?.label,
    title: action.label,
  }))
}

// The nav's own rows, by the id the entries above point at. Owned here rather
// than in the screen because a result has to be able to name the section it
// opens, and because the sections are searchable themselves: typing "updates"
// has to reach the pane called Updates even though no block is called that.
export const SETTING_SECTIONS: readonly { id: string; label: string }[] = [
  { id: "appearance", label: "Appearance" },
  { id: "notifications", label: "Notifications" },
  { id: "hotkeys", label: "Hotkeys" },
  { id: "providers", label: "Providers" },
  { id: "sandbox", label: "Sandbox" },
  { id: "version-control", label: "Version Control" },
  { id: "updates", label: "Updates" },
  { id: "help", label: "Help" },
]

/** The nav label of a section, by id. */
export function sectionLabel(id: string): string {
  return SETTING_SECTIONS.find((section) => section.id === id)?.label ?? id
}

// A section is its own coarsest result. Last in the index, so a block named
// after what was typed outranks the pane it sits in.
function sectionEntries(): SettingEntry[] {
  return SETTING_SECTIONS.map((section) => ({ section: section.id, title: section.label }))
}

/** Every searchable entry: the written-out blocks, the shortcuts, the panes. */
export function allEntries(): SettingEntry[] {
  return [...SETTING_ENTRIES, ...hotkeyEntries(), ...sectionEntries()]
}

/** The entries a query matches, best first.
 *
 * Ranked, not just filtered: a search for "theme" should reach the Theme blocks
 * before "Which sessions run confined" wins on a stray substring. A title that
 * starts with the query beats one that merely contains it, and both beat a
 * match that only came from the `also` words, which the result never shows.
 * Ties keep the order above, which is the order the panes render. */
export function searchSettings<T extends SettingEntry>(query: string, entries: readonly T[]): T[] {
  const needle = query.trim().toLowerCase()
  if (needle === "") {
    return []
  }
  const scored: { entry: T; rank: number }[] = []
  for (const entry of entries) {
    const title = entry.title.toLowerCase()
    if (title.startsWith(needle)) {
      scored.push({ entry, rank: 0 })
    } else if (title.includes(needle)) {
      scored.push({ entry, rank: 1 })
    } else if (entry.also?.includes(needle)) {
      scored.push({ entry, rank: 2 })
    }
  }
  return scored.sort((a, b) => a.rank - b.rank).map((hit) => hit.entry)
}

/** One result, after the per-provider entries have been spread over the
 * providers that are actually enabled. */
export interface SettingHit extends SettingEntry {
  /** Set for a block that lives on a provider's own screen: reaching it means
   * opening the Providers pane and then that provider. */
  providerId?: string
}

/** The index as it applies to this machine: a block that exists once per
 * provider becomes one hit per enabled provider, named after it, and vanishes
 * entirely when none are on. Everything else passes through. */
export function expandEntries(
  providers: readonly { id: string; name: string }[],
  entries: readonly SettingEntry[] = allEntries(),
): SettingHit[] {
  const hits: SettingHit[] = []
  for (const entry of entries) {
    if (!entry.perProvider) {
      hits.push(entry)
      continue
    }
    for (const provider of providers) {
      hits.push({ ...entry, group: provider.name, providerId: provider.id })
    }
  }
  return hits
}

/** The path a result shows above its name, which is what tells two blocks with
 * the same title apart. */
export function hitPath(hit: SettingHit, sectionLabel: string): string {
  return hit.group ? `${sectionLabel} \u203a ${hit.group}` : sectionLabel
}
