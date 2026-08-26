import { describe, expect, it } from "vitest"
import { cannotConfineCopy, type SandboxPlatform } from "./sandbox-copy"

describe("cannotConfineCopy", () => {
  it("keeps the bubblewrap install line on Linux", () => {
    expect(cannotConfineCopy("linux")).toEqual({
      reason: "bubblewrap is not installed",
      advice: "Install bubblewrap and reopen lich. Until then every session runs on the machine.",
    })
  })

  it("names sandbox-exec on macOS", () => {
    expect(cannotConfineCopy("mac")).toEqual({
      reason: "sandbox-exec is not available",
      advice:
        "macOS ships /usr/bin/sandbox-exec, so a machine without a working one is broken in a way lich cannot repair. Every session runs on the machine.",
    })
  })

  it("tells Windows there is no backend rather than offering an install", () => {
    expect(cannotConfineCopy("windows")).toEqual({
      reason: "lich has no sandbox backend on Windows",
      advice: "There is nothing to install — every session runs on the machine.",
    })
  })

  it("does not repeat the reason in the advice on Windows", () => {
    const { reason, advice } = cannotConfineCopy("windows")
    expect(advice).not.toContain(reason)
  })

  it("names bubblewrap on Linux only", () => {
    for (const platform of ["mac", "windows"] satisfies SandboxPlatform[]) {
      const { reason, advice } = cannotConfineCopy(platform)
      expect(`${reason} ${advice}`.toLowerCase()).not.toContain("bubblewrap")
    }
  })

  it("says every session runs on the machine on all three", () => {
    for (const platform of ["linux", "mac", "windows"] satisfies SandboxPlatform[]) {
      expect(cannotConfineCopy(platform).advice.toLowerCase()).toContain(
        "every session runs on the machine",
      )
    }
  })
})
