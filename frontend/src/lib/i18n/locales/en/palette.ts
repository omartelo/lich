// Keys: palette.<component>.<what>, the component in camelCase after its file.
export const palette = {
  commandPalette: {
    title: "Command palette",
    placeholder: "Jump to a session, project or something said, or type > for actions…",
    searchLabel: "Search sessions and projects",
    resultsLabel: "Results",
    filterLabel: "Filter results",
    forgetFailed: "Could not forget {label}: {error}",
    resumeFailed: "Could not resume {label}: {error}",
    actionUnavailable: "{label} is not available here",
    showBeside: "show beside",
    nothingClosed: "Nothing closed yet",
    nothingClosedHint:
      "Close a session and it waits here — its branch, its agent and its conversation — until its worktree is removed.",
    noMatches: "No matches for {query}",
    noMatchesInTab: "No matches for {query} in {tab}",
    shownOfTotal: "{shown} of {total}",
    tab: {
      All: "All",
      Sessions: "Sessions",
      Projects: "Projects",
      Messages: "Messages",
      History: "History",
    },
    checkoutGone: "checkout gone",
    matches: { one: "{count} match", other: "{count} matches" },
    sessions: { one: "{count} session", other: "{count} sessions" },
    relocate: "relocate",
    reopen: "reopen",
  },
} as const
