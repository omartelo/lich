import { describe, expect, it } from "vitest"
import { findLabelMatches, sessionLinkTargets } from "./session-links"
import type { PaletteSession } from "@/lib/session/command-palette"

function session(overrides: Partial<PaletteSession>): PaletteSession {
  return {
    sessionId: "id",
    projectId: "p1",
    projectName: "lich",
    label: "Session",
    kind: "claude",
    path: "/home/me/code/lich",
    ...overrides,
  }
}

describe("sessionLinkTargets", () => {
  it("maps each label to its session", () => {
    const targets = sessionLinkTargets(
      [session({ sessionId: "a", label: "auth" }), session({ sessionId: "b", label: "docs" })],
      "",
    )
    expect(targets.get("auth")?.sessionId).toBe("a")
    expect(targets.get("docs")?.sessionId).toBe("b")
  })

  it("excludes the session doing the printing", () => {
    const targets = sessionLinkTargets([session({ sessionId: "a", label: "auth" })], "a")
    expect(targets.size).toBe(0)
  })

  it("drops a label shared by more than one open session", () => {
    const targets = sessionLinkTargets(
      [
        session({ sessionId: "a", label: "Session 1" }),
        session({ sessionId: "b", label: "Session 1" }),
      ],
      "",
    )
    expect(targets.has("Session 1")).toBe(false)
  })
})

describe("findLabelMatches", () => {
  const targets = sessionLinkTargets(
    [
      session({ sessionId: "a", label: "auth" }),
      session({ sessionId: "b", label: "auth service" }),
    ],
    "",
  )

  it("finds a label appearing in a line of output", () => {
    const matches = findLabelMatches("waiting on auth to finish", targets)
    expect(matches).toEqual([
      { start: 11, end: 15, session: expect.objectContaining({ sessionId: "a" }) },
    ])
  })

  it("finds every occurrence", () => {
    const matches = findLabelMatches("auth then auth again", targets)
    expect(matches.map((m) => m.start)).toEqual([0, 10])
  })

  it("prefers the longer label at the same position", () => {
    const matches = findLabelMatches("ping auth service now", targets)
    expect(matches).toHaveLength(1)
    expect(matches[0]).toMatchObject({ start: 5, end: 17 })
    expect(matches[0].session.sessionId).toBe("b")
  })

  it("is empty with no targets", () => {
    expect(findLabelMatches("auth", new Map())).toEqual([])
  })

  it("is empty for an empty line", () => {
    expect(findLabelMatches("", targets)).toEqual([])
  })

  it("is empty when nothing matches", () => {
    expect(findLabelMatches("unrelated output", targets)).toEqual([])
  })
})
