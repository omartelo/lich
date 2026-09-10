import { describe, expect, it } from "vitest"
import { Terminal } from "@xterm/xterm"
import type { ILink } from "@xterm/xterm"
import { createSessionLinkProvider } from "./session-link-provider"
import { sessionLinkTargets } from "./session-links"
import type { PaletteSession } from "@/lib/session/command-palette"

function provideLinks(
  provider: ReturnType<typeof createSessionLinkProvider>,
  line: number,
): Promise<ILink[] | undefined> {
  return new Promise((resolve) => provider.provideLinks(line, resolve))
}

// @xterm/xterm's Terminal needs no DOM to hold a buffer and run a link
// provider against it — only .open(container) does — so this drives the real
// provider against a real terminal buffer instead of a hand-rolled stand-in,
// without jsdom (vitest here runs in the node environment; see frontend/CLAUDE.md).

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

function writeLine(term: Terminal, text: string): Promise<void> {
  return new Promise((resolve) => term.write(`${text}\r\n`, () => resolve()))
}

describe("createSessionLinkProvider", () => {
  it("finds another open session's label in the terminal's own output", async () => {
    const term = new Terminal({ cols: 80, rows: 24 })
    await writeLine(term, "waiting on auth to finish")

    const targets = sessionLinkTargets([session({ sessionId: "auth-1", label: "auth" })], "")
    const activated: PaletteSession[] = []
    const provider = createSessionLinkProvider(term, { current: targets }, (t) => activated.push(t))

    const links = await provideLinks(provider, 1)
    expect(links).toHaveLength(1)
    const link = (links as ILink[])[0]
    expect(link.text).toBe("auth")
    expect(link.range).toEqual({ start: { x: 12, y: 1 }, end: { x: 15, y: 1 } })

    link.activate({} as MouseEvent, link.text)
    expect(activated).toEqual([expect.objectContaining({ sessionId: "auth-1" })])
  })

  it("finds a label that wrapped across two rows, from either of them", async () => {
    // 20 columns puts "auth service" astride the wrap: "auth " ends row 1 and
    // "service" opens row 2. Read a row at a time the label is on neither.
    const term = new Terminal({ cols: 20, rows: 24 })
    await writeLine(term, "waiting on the auth service")

    const targets = sessionLinkTargets(
      [session({ sessionId: "auth-1", label: "auth service" })],
      "",
    )
    const provider = createSessionLinkProvider(term, { current: targets }, () => {})

    const range = { start: { x: 16, y: 1 }, end: { x: 7, y: 2 } }
    for (const row of [1, 2]) {
      const links = await provideLinks(provider, row)
      expect(links).toHaveLength(1)
      expect((links as ILink[])[0].text).toBe("auth service")
      expect((links as ILink[])[0].range).toEqual(range)
    }
  })

  it("counts cells, not characters, when a wide char sits before the label", async () => {
    // Each of the two wide chars is one character of the line's text and two
    // columns of its buffer, so a match mapped by string offset would underline
    // two cells to the left of the label.
    const term = new Terminal({ cols: 80, rows: 24 })
    await writeLine(term, "日本 auth done")

    const targets = sessionLinkTargets([session({ sessionId: "auth-1", label: "auth" })], "")
    const provider = createSessionLinkProvider(term, { current: targets }, () => {})

    const links = await provideLinks(provider, 1)
    expect(links).toHaveLength(1)
    expect((links as ILink[])[0].range).toEqual({ start: { x: 6, y: 1 }, end: { x: 9, y: 1 } })
  })

  it("counts cells, not characters, when a combining mark sits before the label", async () => {
    // "é" written as e + U+0301 is two characters in one cell, the mirror of the
    // wide char above: mapping by string offset would run one cell too far right.
    const term = new Terminal({ cols: 80, rows: 24 })
    await writeLine(term, "cafe\u0301 auth done")

    const targets = sessionLinkTargets([session({ sessionId: "auth-1", label: "auth" })], "")
    const provider = createSessionLinkProvider(term, { current: targets }, () => {})

    const links = await provideLinks(provider, 1)
    expect(links).toHaveLength(1)
    expect((links as ILink[])[0].range).toEqual({ start: { x: 6, y: 1 }, end: { x: 9, y: 1 } })
  })

  it("reports no links on a line that mentions nothing open", async () => {
    const term = new Terminal({ cols: 80, rows: 24 })
    await writeLine(term, "nothing to see here")

    const targets = sessionLinkTargets([session({ sessionId: "auth-1", label: "auth" })], "")
    const provider = createSessionLinkProvider(term, { current: targets }, () => {})

    expect(await provideLinks(provider, 1)).toBeUndefined()
  })

  it("never links this session's own label back to itself", async () => {
    const term = new Terminal({ cols: 80, rows: 24 })
    await writeLine(term, "this is the auth session")

    // Excluded at the source, the way SessionTargetPicker's own targets are.
    const targets = sessionLinkTargets([session({ sessionId: "self", label: "auth" })], "self")
    const provider = createSessionLinkProvider(term, { current: targets }, () => {})

    expect(await provideLinks(provider, 1)).toBeUndefined()
  })
})
