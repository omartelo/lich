import { readPref, writePref } from "@/lib/prefs"

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

// The pane a screen with nothing remembered opens on: the one almost every
// visit is for.
const DEFAULT_SECTION = "providers"

/** The pane the nav had open.
 *
 * Not checked against a known set the way a sort is. A provider's section id
 * only exists while that provider is enabled, so the set is not knowable here —
 * and the screen already resolves an id it cannot find to its first section,
 * which means a pane belonging to a provider turned off for now is waited out
 * rather than forgotten. */
export function readSettingsSection(): string {
  return readPref(SECTION_KEY) ?? DEFAULT_SECTION
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
