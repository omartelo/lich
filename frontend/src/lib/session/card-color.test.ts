import { describe, expect, it } from "vitest"
import { isCardColor, sharedColor } from "./card-color"
import { addSession, sessionsOf, setSessionsColor, type SessionState } from "./sessions"

const P = "project-1"

function buildState(n: number): SessionState {
  let state: SessionState = addSession({}, P, "s1")
  for (let i = 2; i <= n; i++) {
    state = addSession(state, P, `s${i}`)
  }
  return state
}

describe("setSessionsColor", () => {
  it("paints sessions and hands them back to the theme", () => {
    let state = setSessionsColor(buildState(3), P, ["s1", "s2"], "blue")
    expect(sessionsOf(state, P).map((s) => s.color)).toEqual(["blue", "blue", undefined])
    state = setSessionsColor(state, P, ["s1"], "")
    expect("color" in sessionsOf(state, P)[0]).toBe(false)
  })

  it("ignores an unknown project or session", () => {
    const state = buildState(2)
    expect(setSessionsColor(state, "nope", ["s1"], "red")).toBe(state)
    expect(setSessionsColor(state, P, ["ghost"], "red")).toBe(state)
  })
})

describe("sharedColor", () => {
  it("names the colour every session shares", () => {
    const state = setSessionsColor(buildState(2), P, ["s1", "s2"], "teal")
    expect(sharedColor(sessionsOf(state, P))).toBe("teal")
  })

  it("has none when the colours differ or some card follows the theme", () => {
    const mixed = setSessionsColor(
      setSessionsColor(buildState(2), P, ["s1"], "teal"),
      P,
      ["s2"],
      "red",
    )
    expect(sharedColor(sessionsOf(mixed, P))).toBeUndefined()
    const partial = setSessionsColor(buildState(2), P, ["s1"], "teal")
    expect(sharedColor(sessionsOf(partial, P))).toBeUndefined()
    expect(sharedColor([])).toBeUndefined()
  })
})

describe("isCardColor", () => {
  // A name from a newer lich's palette is drawn as no colour rather than as an
  // empty custom property.
  it("knows the palette and nothing else", () => {
    expect(isCardColor("violet")).toBe(true)
    expect(isCardColor("chartreuse")).toBe(false)
    expect(isCardColor("toString")).toBe(false)
    expect(isCardColor(undefined)).toBe(false)
  })
})
