// Keys: terminal.<component>.<what>, the component in camelCase after its file.
export const terminal = {
  dropHint: {
    attachTo: "Attach to {label}",
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
} as const
