// The three reasons sandbox.Backend() answers "", and the only place the pane is
// told them apart. Only Linux's is about bubblewrap: macOS confines with
// sandbox-exec, and Windows has no backend at all — telling either of them to go
// install a Linux program is an error message about somebody else's machine.
export type SandboxPlatform = "linux" | "mac" | "windows"

export interface CannotConfineCopy {
  // What the status strip says after the em dash: why this machine cannot.
  reason: string
  // The line under it: what, if anything, the user can do about it.
  advice: string
}

export function cannotConfineCopy(platform: SandboxPlatform): CannotConfineCopy {
  if (platform === "windows") {
    return {
      reason: "lich has no sandbox backend on Windows",
      advice: "There is nothing to install — every session runs on the machine.",
    }
  }
  if (platform === "mac") {
    return {
      reason: "sandbox-exec is not available",
      advice:
        "macOS ships /usr/bin/sandbox-exec, so a machine without a working one is broken in a way lich cannot repair. Every session runs on the machine.",
    }
  }
  return {
    reason: "bubblewrap is not installed",
    advice: "Install bubblewrap and reopen lich. Until then every session runs on the machine.",
  }
}
