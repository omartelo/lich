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
} as const
