import { describe, expect, it } from "vitest"
import type { QuotaPlan } from "@/lib/api-types"
import { countOpenSessions, installSummary, planSummary } from "./provider-summary"
import type { SessionState } from "@/lib/session/sessions"

function state(kinds: Record<string, string[]>): SessionState {
  return Object.fromEntries(
    Object.entries(kinds).map(([projectId, list]) => [
      projectId,
      {
        sessions: list.map((kind, index) => ({ id: `${projectId}-${index}`, label: "s", kind })),
        activeId: "",
        nextSeq: list.length + 1,
      },
    ]),
  ) as SessionState
}

describe("counting a provider's open sessions", () => {
  it("counts across every project, not only the one in view", () => {
    const sessions = state({ a: ["claude", "shell"], b: ["claude", "codex"] })

    expect(countOpenSessions(sessions, "claude")).toBe(2)
  })

  it("answers zero for a provider nothing is running", () => {
    expect(countOpenSessions(state({ a: ["claude"] }), "crush")).toBe(0)
  })

  it("answers zero with no projects at all", () => {
    expect(countOpenSessions({}, "claude")).toBe(0)
  })
})

const plan = (over: Partial<QuotaPlan>): QuotaPlan => ({
  provider: "claude",
  name: "Claude",
  status: "ok",
  ...over,
})

describe("the plan reading a row shows", () => {
  it("names the hottest window and how much of it is spent", () => {
    const reading = plan({
      windows: [
        { label: "Session", seconds: 5 * 3600, percent: 62 },
        { label: "Weekly", seconds: 7 * 86400, percent: 41 },
      ],
    })

    expect(planSummary(reading)).toBe("62% of the 5h window")
  })

  // The provider's own verdict wins over the fallback, exactly as it does in
  // the gauge: the row must not name a different window than the readout does.
  it("follows the window the provider marked active", () => {
    const reading = plan({
      windows: [
        { label: "Session", seconds: 5 * 3600, percent: 62 },
        { label: "Weekly", seconds: 7 * 86400, percent: 41, active: true },
      ],
    })

    expect(planSummary(reading)).toBe("41% of the 7d window")
  })

  it("drops the window's length when the provider did not report one", () => {
    const reading = plan({ windows: [{ label: "Session", seconds: 0, percent: 20 }] })

    expect(planSummary(reading)).toBe("20% used")
  })

  // A lapsed login is a fact the row owes the user; an empty string would read
  // as a provider that meters nothing, which is a different fact.
  it("says so when the account is signed out", () => {
    expect(planSummary(plan({ status: "signed-out" }))).toBe("Signed out")
  })

  it("says nothing while there is no reading, or none to be had", () => {
    expect(planSummary(null)).toBe("")
    expect(planSummary(plan({ status: "error" }))).toBe("")
    expect(planSummary(plan({ windows: [] }))).toBe("")
  })
})

describe("installSummary", () => {
  // The row that says "Installed" on a machine with nothing on $PATH is the one
  // that has to say where the binary came from, or the reading looks invented.
  it("names the configured path, and leaves a $PATH hit plain", () => {
    expect(installSummary({ source: "setting" })).toBe("Installed · custom path")
    expect(installSummary({ source: "path" })).toBe("Installed")
  })
})
