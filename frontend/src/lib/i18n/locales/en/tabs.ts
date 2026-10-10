// Keys: tabs.<component>.<what>, the component in camelCase after its file.
export const tabs = {
  homeTab: {
    home: "Home",
  },
  notificationsButton: {
    title: "Notifications",
    titlePending: "Notifications, {count} pending",
    caughtUp: "You're all caught up",
    dismiss: "Dismiss {name}",
  },
  openProjectMenu: {
    open: "Open project",
    recent: "Recent projects",
    relocate: "relocate",
    openFolder: "Open folder…",
    moreClosed: {
      one: "{count} more closed project — search the palette",
      other: "{count} more closed projects — search the palette",
    },
  },
  projectTab: {
    close: "Close {name}",
  },
  projectTabs: {
    pullRequests: "Pull requests",
    settings: "Settings",
    closeTitle: "Sessions are still running",
    closeBody: {
      one: "Closing {name} stops a session mid-turn. Reopening the project offers to resume where each left off; the turn in flight is lost.",
      other:
        "Closing {name} stops {count} sessions mid-turn. Reopening the project offers to resume where each left off; the turn in flight is lost.",
    },
    closeAnyway: "Close anyway",
  },
} as const
