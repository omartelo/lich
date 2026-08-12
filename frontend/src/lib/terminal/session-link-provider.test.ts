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
