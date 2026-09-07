import { parseEnumPref, readPref, writePref } from "@/lib/prefs"

// Everything the settings screen remembers about how it was left. Like the pull
// request screen it is an `<Outlet>` route, so stepping into a session or
// another project unmounts it: however deep into a provider's configuration you
// were, coming back landed on the Providers pane with an empty search box.
//
// UI preferences, so the page's localStorage rather than the workspace database
// (see the root CLAUDE.md). pulls/pulls-prefs.ts is the worked example, and the
// split prefs.ts describes applies here too — the parsing is the half worth
// testing, reading and writing the key stays a two-line wrapper.
//
// Both of these are global, and the pull request screen's own rule is what puts
// them there: what narrows a list of a *repository's* content is keyed per
// project, and nothing here does. This route carries a project id, but the nav
// it draws is the same list of panes in every project — the global sections, the
// enabled providers, the footer — so which pane was open and what was typed to
// find it are habits of this user. The project-scoped half of this screen is the
// values the panes read and write, and those already live in the workspace
// database under the project's own id.
const SECTION_KEY = "lich.settings.section"
const QUERY_KEY = "lich.settings.query"
const PROVIDER_KEY = "lich.settings.provider"

// The pane a screen with nothing remembered opens on: the one almost every
// visit is for. Also where a section id this build cannot place lands, and a
// pane that no longer exists must not strand its readers.
export const DEFAULT_SECTION = "providers"

/** The pane the nav had open, parsed against the sections the screen has.
 *
 * The panes belong to the screen, so it passes its own ids in — but they are a
 * known set at the moment of the read, which is what makes this a parse like
 * any other sort or theme. An id from a build that shaped the nav differently
 * (a `provider-<id>` pane, from before Providers became one section) opens the
 * default and is rewritten, rather than being resolved again on every launch
 * for the life of the install. */
export function readSettingsSection(sections: readonly string[]): string {
  const stored = readPref(SECTION_KEY)
  const section = parseEnumPref(stored, sections, DEFAULT_SECTION)
  if (stored !== null && stored !== section) {
    writePref(SECTION_KEY, section)
  }
  return section
}

export function writeSettingsSection(section: string): void {
  writePref(SECTION_KEY, section)
}

/** The search box, verbatim — it is free text, so anything stored is valid.
 * Stored per keystroke on purpose, as the pull request screen's filter box is:
 * a screen left mid-search is a screen that comes back mid-search. */
export function readSettingsQuery(): string {
  return readPref(QUERY_KEY) ?? ""
}

export function writeSettingsQuery(query: string): void {
  writePref(QUERY_KEY, query)
}

/** The provider whose own screen was open inside the Providers pane, "" for the
 * list. Unparsed, unlike the section: the roster is the backend's answer and is
 * not in yet when this is read, and an id is only meaningful while that
 * provider is enabled anyway. So the pane resolves it — back to the list while
 * the provider is off, to its screen again when it is turned on — and the
 * stored id is kept on purpose, which is what makes it come back. */
export function readSettingsProvider(): string {
  return readPref(PROVIDER_KEY) ?? ""
}

export function writeSettingsProvider(provider: string): void {
  writePref(PROVIDER_KEY, provider)
}
