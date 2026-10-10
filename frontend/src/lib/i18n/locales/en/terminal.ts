// Keys: terminal.<file>.<what>, named after the component or the lib/terminal module.
export const terminal = {
  dropHint: {
    attachTo: "Attach to {label}",
    confined: "Outside the checkout it arrives as a copy",
    pasted: "Its path is pasted at the prompt",
  },
  exitBanner: {
    restart: "Restart",
    close: "Close",
  },
  searchBar: {
    placeholder: "Find",
    label: "Search terminal",
    previous: "Previous match",
    next: "Next match",
    close: "Close search",
  },
  drop: {
    notAttached: "Not attached: {files}",
  },
  host: {
    stopShowing: "Stop showing {name}",
    paneName: "The {name} pane",
    rememberFailed: "Couldn't remember the choice: {error}",
    showBeside: "Show a session beside this one",
    noRoom: "No room for another pane",
  },
  showBesidePicker: {
    title: "Show beside",
    placeholder: "Search sessions to show beside…",
    search: "Search sessions to show beside",
    results: "Sessions that can be shown beside",
    pick: "show",
    noMatch: "No sessions match {query}",
    onWall: "on {group}",
  },
  view: {
    restartFailed: "Session failed to restart: {error}",
    startFailed: "Session failed to start: {error}",
    pasteSettingFailed:
      "Could not read whether to unfold long pastes, so this one was pasted once: {error}",
  },
  sessionExit: {
    ended: "Session ended",
    endedWithCode: "Session ended with code {code}",
  },
  dropFiles: {
    folderConfined:
      "folders outside this sandboxed session's checkout cannot be handed over; drop files",
    folderMissing: "folder not found under this session or your home; drop files",
  },
  copyToast: {
    copied: {
      one: "copied {count} char to clipboard",
      other: "copied {count} chars to clipboard",
    },
  },
} as const
