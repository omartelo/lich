// Keys: session.<module>.<what>, the module in camelCase after its file in lib/session.
export const session = {
  schedule: {
    choice15Minutes: "15 min",
    choice1Hour: "1 hour",
    choice4Hours: "4 hours",
    choiceTomorrow: "Tomorrow",
    countdown: "in {amount}",
    today: "today {clock}",
    weekdayAt: "{weekday} {clock}",
  },
  limitLine: {
    sessionWindow: "Session limit",
    weeklyWindow: "Weekly limit",
    usageWindow: "Usage limit",
  },
  filter: {
    phase: {
      waiting: "waiting",
      running: "running",
      unread: "unread",
      idle: "idle",
    },
    phaseJoiner: " or ",
    noMatch: "No sessions. The active session stays.",
    noMatchQuery: "No sessions match “{query}”. The active session stays.",
    noMatchPhases: "No {phases} sessions. The active session stays.",
    noMatchPhasesQuery: "No {phases} sessions match “{query}”. The active session stays.",
  },
  cost: {
    mixedModels: "This conversation switched models, so lich cannot price it.",
    unpricedModel: "No price for this model yet — lich needs the network to fetch one.",
  },
  handsOn: {
    detailTurn:
      "How long this session has been worked on — typed at, reporting, or running a turn. A gap longer than 15 minutes counts as time away.",
    detailTool:
      "How long this session has been worked on — typed at, or reporting a tool call. A gap longer than 15 minutes counts as time away.",
  },
  spawnGate: {
    checkoutGone:
      "This session's worktree is gone, so the session was closed. Re-create the worktree to pick it up again.",
    conversationGone: "The previous conversation is no longer available — starting a new session.",
  },
  fork: {
    unavailable: "{name} keeps no fork; resume only.",
  },
  palette: {
    untitledConversation: "Untitled conversation",
    groupSessions: "Sessions",
    groupProjects: "Projects",
    groupMessages: "Messages",
    groupOpen: "Open",
    groupClosed: "Closed",
    groupClosedSessions: "Closed sessions",
    groupOutsideLich: "Outside lich",
    indexing: { one: "indexing {count} session", other: "indexing {count} sessions" },
  },
} as const
