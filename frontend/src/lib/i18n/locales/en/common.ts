// Words more than one folder uses. A message moves here only once a second
// folder needs it; until then it lives in its folder's namespace.
export const common = {
  count: {
    file: { one: "{count} file", other: "{count} files" },
    commit: { one: "{count} commit", other: "{count} commits" },
  },
  time: {
    justNow: "just now",
  },
  action: {
    cancel: "Cancel",
    save: "Save",
    create: "Create",
  },
  checkAgainButton: {
    check: "Check again",
    checking: "Checking…",
  },
  errorBoundary: {
    stoppedRendering: "{label} stopped rendering",
    reload: "Reload the window",
    retry: "Try again",
  },
  stepper: {
    default: "Default",
  },
  toolMissing: {
    notInstalled: "{label} is not installed",
    install: "Install {bin}",
  },
  pickerDialog: {
    navigate: "navigate",
    filter: "filter",
    close: "close",
  },
  ui: {
    dialog: {
      close: "Close",
    },
  },
} as const
