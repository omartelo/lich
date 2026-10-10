// What the settings search looks through. The nav has eight labels; the things
// people actually look for are the two dozen controls under them, and until
// this existed a search for "theme" or "ssh" answered that nothing matched.
//
// The index is written out rather than derived from the rendered tree: the
// panes are React components whose blocks only exist once rendered, and the
// suite runs in node. settings-index.test.ts is what keeps the two in step, by
// reading the same titles out of the source (a literal, or the catalog key a
// translated block passes to t()) and failing on a block nobody added here.
//
// A translated block is indexed by its key, resolved when the search runs: it
// matches in the interface language, and in English too, so a search typed
// from memory of the English screens still lands after a language switch.
import { HOTKEY_ACTIONS, HOTKEY_GROUPS } from "@/lib/hotkeys"
import { type PlainMessageKey, t, tIn } from "@/lib/i18n/i18n"

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
  /** Narrows a per-provider block to the one provider whose screen has it. */
  onlyFor?: string
  /** A translated entry's English title and keywords, which it also matches. */
  english?: string
}

/** An entry as written in the index: a block still titled with an English
 * literal, or a translated one named by the catalog keys its pane renders. */
export type IndexedSetting = Omit<SettingEntry, "title" | "also" | "english"> &
  ({ title: string; also?: string } | { titleKey: PlainMessageKey; alsoKey?: PlainMessageKey })

// Every SettingBlock in the app, in the order its pane renders it.
export const SETTING_ENTRIES: readonly IndexedSetting[] = [
  {
    section: "appearance",
    titleKey: "settings.themePicker.title",
    alsoKey: "settings.themePicker.searchWords",
  },
  { section: "appearance", titleKey: "settings.appearanceSettings.zoomTitle" },
  { section: "appearance", titleKey: "settings.appearanceSettings.textSizeTitle" },
  {
    section: "appearance",
    titleKey: "settings.fontSetting.title",
    alsoKey: "settings.fontSetting.searchWords",
  },
  {
    section: "appearance",
    titleKey: "settings.footerSettings.title",
    alsoKey: "settings.footerSettings.searchWords",
  },
  {
    section: "appearance",
    titleKey: "settings.footerSettings.ceilingTitle",
    alsoKey: "settings.footerSettings.ceilingSearchWords",
  },
  {
    section: "appearance",
    titleKey: "settings.language.uiTitle",
    alsoKey: "settings.language.uiSearchWords",
  },
  {
    section: "appearance",
    titleKey: "settings.language.promptTitle",
    alsoKey: "settings.language.promptSearchWords",
  },
  {
    section: "notifications",
    titleKey: "settings.notificationsSettings.needsInputTitle",
  },
  {
    section: "notifications",
    titleKey: "settings.notificationsSettings.finishedTitle",
  },
  {
    section: "providers",
    titleKey: "settings.providersPane.defaultTitle",
    alsoKey: "settings.providersPane.defaultSearchWords",
  },
  {
    section: "providers",
    titleKey: "settings.planUsageSetting.title",
    alsoKey: "settings.planUsageSetting.searchWords",
    perProvider: true,
  },
  {
    section: "providers",
    titleKey: "settings.providerBinary.title",
    alsoKey: "settings.providerBinary.searchWords",
    perProvider: true,
  },
  {
    section: "providers",
    titleKey: "settings.providerBinSettings.skipTitle",
    alsoKey: "settings.providerBinSettings.skipSearchWords",
    perProvider: true,
  },
  {
    section: "providers",
    titleKey: "settings.ultracodeSetting.title",
    alsoKey: "settings.ultracodeSetting.searchWords",
    perProvider: true,
    onlyFor: "claude",
  },
  {
    section: "providers",
    titleKey: "settings.subagentCardsSetting.title",
    alsoKey: "settings.subagentCardsSetting.searchWords",
    perProvider: true,
    onlyFor: "claude",
  },
  {
    section: "providers",
    titleKey: "settings.restoreSetting.title",
    alsoKey: "settings.restoreSetting.searchWords",
    perProvider: true,
  },
  {
    section: "providers",
    titleKey: "settings.providerDetail.rightNowTitle",
    alsoKey: "settings.providerDetail.rightNowSearchWords",
    perProvider: true,
  },
  {
    section: "sandbox",
    titleKey: "settings.sandboxSettings.confinedTitle",
    alsoKey: "settings.sandboxSettings.confinedSearchWords",
  },
  { section: "sandbox", titleKey: "settings.sandboxSettings.carryTitle" },
  {
    section: "sandbox",
    titleKey: "settings.sandboxSettings.sshTitle",
    alsoKey: "settings.sandboxSettings.sshSearchWords",
  },
  {
    section: "sandbox",
    titleKey: "settings.sandboxSettings.ghTitle",
    alsoKey: "settings.sandboxSettings.ghSearchWords",
  },
  {
    section: "version-control",
    titleKey: "settings.vcsToolsSetting.title",
    alsoKey: "settings.vcsToolsSetting.searchWords",
  },
  {
    section: "version-control",
    titleKey: "settings.diffLayoutSetting.title",
    alsoKey: "settings.diffLayoutSetting.searchWords",
  },
  {
    section: "version-control",
    title: "GitHub account",
    also: "login gh switch",
  },
  {
    section: "updates",
    titleKey: "settings.updatesSettings.applicationTitle",
    alsoKey: "settings.updatesSettings.applicationSearchWords",
  },
  {
    section: "updates",
    titleKey: "settings.updatesSettings.whatsNewTitle",
    alsoKey: "settings.updatesSettings.whatsNewSearchWords",
  },
  {
    section: "updates",
    titleKey: "settings.pluginSetting.title",
    alsoKey: "settings.pluginSetting.searchWords",
  },
  {
    section: "help",
    titleKey: "settings.helpSettings.bugTitle",
    alsoKey: "settings.helpSettings.bugSearchWords",
  },
  {
    section: "help",
    titleKey: "settings.helpSettings.logTitle",
    alsoKey: "settings.helpSettings.logSearchWords",
  },
  {
    section: "help",
    titleKey: "settings.helpSettings.aboutTitle",
    alsoKey: "settings.helpSettings.aboutSearchWords",
  },
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
export interface SettingSection {
  id: string
  labelKey: PlainMessageKey
  /** The nav label in the current language, read when asked rather than when
   * the module loads. */
  readonly label: string
}

function section(id: string, labelKey: PlainMessageKey): SettingSection {
  return {
    id,
    labelKey,
    get label() {
      return t(labelKey)
    },
  }
}

export const SETTING_SECTIONS: readonly SettingSection[] = [
  section("appearance", "settings.settings.section.appearance"),
  section("notifications", "settings.settings.section.notifications"),
  section("hotkeys", "settings.settings.section.hotkeys"),
  section("providers", "settings.settings.section.providers"),
  section("sandbox", "settings.settings.section.sandbox"),
  section("version-control", "settings.settings.section.versionControl"),
  section("updates", "settings.settings.section.updates"),
  section("help", "settings.settings.section.help"),
]

/** The nav label of a section, by id, in the current language. */
export function sectionLabel(id: string): string {
  return SETTING_SECTIONS.find((candidate) => candidate.id === id)?.label ?? id
}

// A section is its own coarsest result. Last in the index, so a block named
// after what was typed outranks the pane it sits in.
function sectionEntries(): SettingEntry[] {
  return SETTING_SECTIONS.map((section) => ({
    section: section.id,
    title: t(section.labelKey),
    english: tIn("en", section.labelKey).toLowerCase(),
  }))
}

/** An entry with its keys read in the current language. Called at search
 * time, never at import, so a language switch reaches the next search. */
function resolveEntry(entry: IndexedSetting): SettingEntry {
  if ("title" in entry) {
    return entry
  }
  const { titleKey, alsoKey, ...rest } = entry
  return {
    ...rest,
    title: t(titleKey),
    also: alsoKey && t(alsoKey),
    english: [tIn("en", titleKey), alsoKey && tIn("en", alsoKey)].join(" ").toLowerCase(),
  }
}

/** Every searchable entry: the written-out blocks, the shortcuts, the panes. */
export function allEntries(): SettingEntry[] {
  return [...SETTING_ENTRIES.map(resolveEntry), ...hotkeyEntries(), ...sectionEntries()]
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
    } else if (entry.also?.toLowerCase().includes(needle) || entry.english?.includes(needle)) {
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
      if (entry.onlyFor && entry.onlyFor !== provider.id) {
        continue
      }
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
