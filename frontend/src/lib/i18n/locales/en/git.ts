// Keys: git.<module>.<what>, the module in camelCase after its file in lib/git.
export const git = {
  baseStatus: {
    behind: {
      one: "{count} commit behind {base}",
      other: "{count} commits behind {base}",
    },
    conflict: { one: "Conflicts in {count} file", other: "Conflicts in {count} files" },
  },
  fileSearch: {
    cutMatches: "{count}+ matches",
    summary: "{matches} in {files}",
    matches: { one: "{count} match", other: "{count} matches" },
    cutNote: "Showing the first {count} matching lines. Narrow the search.",
    tooLarge: {
      one: "{count} file over 1 MB not searched.",
      other: "{count} files over 1 MB not searched.",
    },
  },
  fileTree: {
    hidden: "Hidden: {names}.",
    cut: "This folder has more files than the tree can list.",
  },
  lastTurn: {
    saidPreviousTurn: "from the previous turn",
    noTurnWindow:
      "{name} reports neither the start nor the end of a turn, so there is no window to bracket.",
  },
  carry: {
    reused:
      "{name} already existed, so it was checked out as it stands — the uncommitted work was not carried over.",
    failed: "Couldn’t carry the uncommitted work over: {error}",
  },
  codemirror: {
    expand: { one: "Expand {count} unchanged line", other: "Expand {count} unchanged lines" },
    showLines: "Show lines {from}–{to}",
    revertTitle: "Revert this change",
    revert: "Revert",
  },
} as const
