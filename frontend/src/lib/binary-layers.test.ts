import { describe, expect, it } from "vitest"
import type { BinaryCheck } from "./api-types"
import {
  checkDetail,
  checkLabel,
  failed,
  parkedLabel,
  resolves,
  winningScope,
} from "./binary-layers"

const check = (status: BinaryCheck["status"], path = ""): BinaryCheck => ({ path, status })

const layer = (bin: string, off = false) => ({ bin, off })

describe("winningScope", () => {
  it("prefers the project override, then the global one", () => {
    expect(winningScope(layer("/opt/next/claude"), layer("/opt/claude"))).toBe("project")
    expect(winningScope(layer(""), layer("/opt/claude"))).toBe("global")
    expect(winningScope(layer(""), layer(""))).toBe("path")
  })

  // A field holding spaces is a field nobody filled in — and the Go side trims
  // before storing, so a value that reached the store this way is whitespace
  // from an older build.
  it("treats a whitespace-only value as unset", () => {
    expect(winningScope(layer("   "), layer(""))).toBe("path")
    expect(winningScope(layer(" \t "), layer("/opt/claude"))).toBe("global")
  })

  // The switch is the whole feature: the path stays written and stops counting,
  // so falling back a layer costs a click rather than the path.
  it("skips a parked layer while it keeps its path", () => {
    expect(winningScope(layer("/opt/next/claude", true), layer("/opt/claude"))).toBe("global")
    expect(winningScope(layer("/opt/next/claude", true), layer("/opt/claude", true))).toBe("path")
    expect(winningScope(layer("", true), layer("/opt/claude"))).toBe("global")
  })
})

describe("resolves", () => {
  it("is true only for a switched-on layer holding a path", () => {
    expect(resolves(layer("/opt/claude"))).toBe(true)
    expect(resolves(layer("/opt/claude", true))).toBe(false)
    expect(resolves(layer(""))).toBe(false)
    expect(resolves(layer("  ", false))).toBe(false)
  })
})

describe("parkedLabel", () => {
  const off = (bin: string) => layer(bin, true)
  const unset = layer("")

  it("says nothing when nothing is parked", () => {
    expect(parkedLabel("path", unset, unset, "myproject")).toBe("")
    expect(parkedLabel("project", layer("/opt/next/claude"), unset, "myproject")).toBe("")
  })

  // The repro this guard exists for: switching the override off writes the flag,
  // then clearing the field writes an empty path over it. The flag outlives the
  // path, and the closed state used to keep announcing an override nobody set.
  it("does not claim an override for a flag with no path behind it", () => {
    expect(parkedLabel("path", off(""), off(""), "myproject")).toBe("")
    expect(parkedLabel("path", off("   "), off(" \t "), "myproject")).toBe("")
  })

  it("names a parked override the winner did not shadow", () => {
    expect(parkedLabel("path", off("/opt/next/claude"), unset, "myproject")).toBe(
      " · myproject override off",
    )
    expect(parkedLabel("global", off("/opt/next/claude"), layer("/opt/claude"), "myproject")).toBe(
      " · myproject override off",
    )
    expect(parkedLabel("path", unset, off("/opt/claude"), "myproject")).toBe(
      " · global override off",
    )
  })

  // Both parked is why $PATH won, and either switch flipped back on would change
  // what spawns — so the closed line owns up to both rather than the nearer one.
  it("names both when both are parked", () => {
    expect(parkedLabel("path", off("/opt/next/claude"), off("/opt/claude"), "myproject")).toBe(
      " · myproject override off · global override off",
    )
  })

  // A parked layer under a winning override is not news: its switch changes
  // nothing while the layer above it resolves.
  it("stays quiet about a layer the winner shadows", () => {
    expect(parkedLabel("project", layer("/opt/next/claude"), off("/opt/claude"), "myproject")).toBe(
      "",
    )
  })

  // The pane can hold a project override before the project itself has loaded.
  it("falls back to a generic name when the project has none yet", () => {
    expect(parkedLabel("path", off("/opt/next/claude"), unset, undefined)).toBe(
      " · project override off",
    )
  })
})

describe("checkLabel", () => {
  it("says how a missing binary was looked for", () => {
    expect(checkLabel(check("not-found"), "claude")).toBe("not on $PATH")
    expect(checkLabel(check("not-found"), "/opt/gone/claude")).toBe("no such file")
    expect(checkLabel(check("not-found"), "C:\\tools\\claude.exe")).toBe("no such file")
  })

  it("names the two mistakes a shell would have hidden", () => {
    expect(checkLabel(check("home-shortcut"), "~/dev/claude")).toBe("~ is not expanded")
    expect(checkLabel(check("relative"), "bin/claude")).toBe("relative path")
  })

  it("says nothing for a check that has not arrived or has nothing to report", () => {
    expect(checkLabel(null, "claude")).toBe("")
    expect(checkLabel(check("empty"), "")).toBe("")
  })

  it("confirms a binary that resolves", () => {
    expect(checkLabel(check("ok", "/usr/bin/claude"), "claude")).toBe("executable")
  })
})

describe("checkDetail", () => {
  it("explains only the states that need fixing", () => {
    expect(checkDetail(check("home-shortcut"))).toContain("taken literally")
    expect(checkDetail(check("relative"))).toContain("per session")
    expect(checkDetail(check("not-found"))).toContain("will not start")
    expect(checkDetail(check("not-executable"))).toContain("will not start")
    expect(checkDetail(check("ok"))).toBe("")
    expect(checkDetail(null)).toBe("")
  })
})

describe("failed", () => {
  // A check in flight is not a failure: drawn as one, a field being typed into
  // would flash red between keystrokes.
  it("is false while a check is in flight, and for an unset layer", () => {
    expect(failed(null)).toBe(false)
    expect(failed(check("empty"))).toBe(false)
  })

  it("is true for every state a session cannot spawn from", () => {
    for (const status of ["not-found", "not-executable", "home-shortcut", "relative"] as const) {
      expect(failed(check(status))).toBe(true)
    }
    expect(failed(check("ok"))).toBe(false)
  })
})
