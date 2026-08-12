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
    const { byLabel } = sessionLinkTargets(
      [session({ sessionId: "a", label: "auth" }), session({ sessionId: "b", label: "docs" })],
      "",
    )
    expect(byLabel.get("auth")?.sessionId).toBe("a")
    expect(byLabel.get("docs")?.sessionId).toBe("b")
  })

  it("excludes the session doing the printing", () => {
    const { byLabel, pattern } = sessionLinkTargets(
      [session({ sessionId: "a", label: "auth" })],
      "a",
    )
    expect(byLabel.size).toBe(0)
    expect(pattern).toBeNull()
  })

  it("drops a label shared by more than one open session", () => {
    const { byLabel } = sessionLinkTargets(
      [
        session({ sessionId: "a", label: "Session 1" }),
        session({ sessionId: "b", label: "Session 1" }),
      ],
      "",
    )
    expect(byLabel.has("Session 1")).toBe(false)
  })

  // An empty alternative matches the empty string at every position without
  // advancing lastIndex, so the exec loop below would never end.
  it("drops an empty label instead of building a pattern that never advances", () => {
    const { byLabel, pattern } = sessionLinkTargets(
      [session({ sessionId: "a", label: "" }), session({ sessionId: "b", label: "docs" })],
      "",
    )
    expect(byLabel.has("")).toBe(false)
    expect(findLabelMatches("anything at all", { byLabel, pattern })).toEqual([])
  })

  it("has no pattern when every session was excluded", () => {
    expect(sessionLinkTargets([], "").pattern).toBeNull()
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

  it("falls back to the shorter label when the longer one runs into a word", () => {
    const matches = findLabelMatches("ping auth serviceable now", targets)
    expect(matches).toHaveLength(1)
    expect(matches[0]).toMatchObject({ start: 5, end: 9 })
    expect(matches[0].session.sessionId).toBe("a")
  })

  it("never links a label glued to a neighbouring word", () => {
    expect(findLabelMatches("run authentication now", targets)).toEqual([])
    expect(findLabelMatches("reauth the client", targets)).toEqual([])
    expect(findLabelMatches("auth-service is up", targets)).toEqual([])
  })

  it("never links a label that is part of a path or a filename", () => {
    expect(findLabelMatches("open src/auth.ts", targets)).toEqual([])
    expect(findLabelMatches("open auth.ts", targets)).toEqual([])
    expect(findLabelMatches("open auth/main.go", targets)).toEqual([])
    expect(findLabelMatches("open C:\\auth\\main.go", targets)).toEqual([])
  })

  it("still links a label that ends a sentence or is punctuated", () => {
    expect(findLabelMatches("ask auth.", targets).map((m) => m.start)).toEqual([4])
    expect(findLabelMatches("ask auth, then wait", targets).map((m) => m.start)).toEqual([4])
    expect(findLabelMatches('ask "auth" first', targets).map((m) => m.start)).toEqual([5])
  })

  it("walks a fresh line from the top, not from where the last one ended", () => {
    expect(findLabelMatches("later on auth here", targets).map((m) => m.start)).toEqual([9])
    expect(findLabelMatches("auth", targets).map((m) => m.start)).toEqual([0])
  })

  it("is empty with no targets", () => {
    expect(findLabelMatches("auth", sessionLinkTargets([], ""))).toEqual([])
  })

  it("is empty for an empty line", () => {
    expect(findLabelMatches("", targets)).toEqual([])
  })

  it("is empty when nothing matches", () => {
    expect(findLabelMatches("unrelated output", targets)).toEqual([])
  })
})
