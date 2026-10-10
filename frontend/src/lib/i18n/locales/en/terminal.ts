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
  },
  view: {
    restartFailed: "Session failed to restart: {error}",
    startFailed: "Session failed to start: {error}",
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
