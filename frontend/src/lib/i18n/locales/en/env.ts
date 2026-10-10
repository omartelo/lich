// What lich says about the machine it runs on: provider binaries, sandbox,
// VCS tools, commit identity. Keys: env.<module>.<what>.
export const env = {
  providerSummary: {
    installed: "Installed",
    installedCustomPath: "Installed · custom path",
    signedOut: "Signed out",
    windowPercent: "{percent}% of the {length} window",
    percentUsed: "{percent}% used",
  },
  quota: {
    tokenLogin: "Token login",
  },
  paths: {
    cwdUnknown: "cwd unknown · inside {host}",
  },
  sandbox: {
    windowsReason: "lich has no sandbox backend on Windows",
    windowsAdvice: "There is nothing to install — every session runs on the machine.",
    macReason: "sandbox-exec is not available",
    macAdvice:
      "macOS ships /usr/bin/sandbox-exec, so a machine without a working one is broken in a way lich cannot repair. Every session runs on the machine.",
    linuxReason: "bubblewrap is not installed",
    linuxAdvice:
      "Install bubblewrap and reopen lich. Until then every session runs on the machine.",
    confinedMeans:
      "An empty home holding only the agent's own state, the machine read-only, and writes only inside its checkout. The network stays on.",
  },
  vcsTools: {
    gitWithout: "Branches, diffs and worktrees stay empty without it.",
    ghWithout: "Pull requests, checks and PR checkouts are unavailable without it.",
  },
  binary: {
    parkedProject: "{name} override off",
    parkedProjectFallback: "project",
    parkedGlobal: "global override off",
    executable: "executable",
    noSuchFile: "no such file",
    notOnPath: "not on $PATH",
    notExecutable: "not executable",
    homeNotExpanded: "~ is not expanded",
    relativePath: "relative path",
    detailHomeShortcut:
      "lich spawns the binary directly, so ~ is taken literally rather than expanded. Use the full path.",
    detailRelative:
      "A relative path is resolved against each session's own working directory, so it names a different binary per session. Use a full path.",
    detailBroken:
      "Sessions will not start until this is fixed. An override can also be switched off or cleared, falling back to the layer below.",
  },
  commitIdentity: {
    noneLead: "No git identity in this checkout.",
    noneNote: "Commits will be refused until user.email is set.",
    landAsNamed: "Commits land as {name}",
    landAs: "Commits land as",
    setLocal: "— set in this repository, overriding your global one.",
    setGlobal: "— git user.email, not this account.",
  },
} as const
