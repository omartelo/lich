import { describe, expect, it } from "vitest"
import { pwshQuote, sendCommand, shQuote } from "./send-command"
import type { Session, SessionState } from "./sessions"

const PROJECTS = [
  { id: "p1", name: "lich", path: "/home/me/code/lich" },
  { id: "p2", name: "revu", path: "/home/me/code/revu" },
]

const auth: Session = { id: "aaaa1111", label: "auth", kind: "claude" }
const docs: Session = { id: "bbbb2222", label: "docs", kind: "codex" }
const otherAuth: Session = { id: "cccc3333", label: "Auth", kind: "crush" }

const state = (p2: Session[]): SessionState => ({
  p1: { activeId: auth.id, nextSeq: 3, sessions: [auth, docs] },
  p2: { activeId: "", nextSeq: 2, sessions: p2 },
})

// The Go half is internal/shquote; these are its cases, so a divergence shows
// up as a failure here rather than as a command that runs the wrong thing.
describe("shQuote", () => {
  it("survives the shell round trip's edge cases", () => {
    expect(shQuote("plain")).toBe("'plain'")
    expect(shQuote("with space")).toBe("'with space'")
    expect(shQuote("it's")).toBe(`'it'\\''s'`)
    expect(shQuote("")).toBe("''")
    expect(shQuote("$HOME `id` ;rm")).toBe("'$HOME `id` ;rm'")
  })
})

// The same cases against the other rule, because lich opens its own Windows
// sessions in PowerShell: single quotes expand nothing there either, and only
// the escape differs. Mirrors internal/shquote's TestQuotePwsh.
describe("pwshQuote", () => {
  it("doubles the quote instead of closing the run", () => {
    expect(pwshQuote("plain")).toBe("'plain'")
    expect(pwshQuote("with space")).toBe("'with space'")
    expect(pwshQuote("it's")).toBe("'it''s'")
    expect(pwshQuote("")).toBe("''")
    expect(pwshQuote("$HOME `id` ;rm")).toBe("'$HOME `id` ;rm'")
  })
})

describe("sendCommand", () => {
  it("names the session alone while its label is unambiguous", () => {
    expect(sendCommand(PROJECTS, state([]), auth)).toBe(`lich send 'auth' "<prompt>"`)
  })

  // --project is what the CLI disambiguates with, and lich send matches a label
  // case-insensitively — so a differently-cased twin is a twin.
  it("narrows to the project once another session holds the label", () => {
    const command = sendCommand(PROJECTS, state([otherAuth]), auth)
    expect(command).toBe(`lich send --project 'lich' 'auth' "<prompt>"`)
    expect(sendCommand(PROJECTS, state([otherAuth]), otherAuth)).toBe(
      `lich send --project 'revu' 'Auth' "<prompt>"`,
    )
  })

  it("leaves a sibling in the same project out of it", () => {
    expect(sendCommand(PROJECTS, state([]), docs)).toBe(`lich send 'docs' "<prompt>"`)
  })

  // The label is the user's own text: a card renamed to something with a quote
  // or a metacharacter in it still has to paste as one argument.
  // On Windows the line is pasted into PowerShell, where the POSIX escape is
  // four literal characters and the argument ends at the first apostrophe.
  it("spells the same label for PowerShell", () => {
    const awkward: Session = { id: "dddd4444", label: `the $PATH 'bug'`, kind: "claude" }
    expect(sendCommand(PROJECTS, state([awkward]), awkward, true)).toBe(
      `lich send 'the $PATH ''bug''' "<prompt>"`,
    )
    expect(sendCommand(PROJECTS, state([otherAuth]), auth, true)).toBe(
      `lich send --project 'lich' 'auth' "<prompt>"`,
    )
  })

  it("quotes a label the shell would otherwise read", () => {
    const awkward: Session = { id: "dddd4444", label: `the $PATH 'bug'`, kind: "claude" }
    expect(sendCommand(PROJECTS, state([awkward]), awkward)).toBe(
      `lich send 'the $PATH '\\''bug'\\''' "<prompt>"`,
    )
  })

  // A session whose project is not in the list has no project to name; the line
  // still addresses it, because the label alone is what lich send resolves.
  it("still writes a line for a session no listed project holds", () => {
    const stray: Session = { id: "eeee5555", label: "stray", kind: "shell" }
    expect(sendCommand(PROJECTS, state([]), stray)).toBe(`lich send 'stray' "<prompt>"`)
  })
})
