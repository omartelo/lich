import { describe, expect, it } from "vitest"
import { Terminal } from "@xterm/xterm"
import { colorSchemeReport, watchThemeNotify } from "./theme-notify"

// A real xterm parser, no DOM: only .open(container) needs one.
function rig(scheme: "light" | "dark" = "light") {
  const term = new Terminal({ cols: 80, rows: 24, allowProposedApi: true })
  const state = { themeNotify: false }
  const replies: string[] = []
  const watch = watchThemeNotify(
    term,
    state,
    () => scheme,
    (data) => replies.push(data),
  )
  const write = (data: string) => new Promise<void>((resolve) => term.write(data, resolve))
  return { term, state, replies, watch, write }
}

describe("watchThemeNotify", () => {
  it("turns on with DECSET 2031 and off with DECRST 2031", async () => {
    const { state, write } = rig()
    await write("\x1b[?2031h")
    expect(state.themeNotify).toBe(true)
    await write("\x1b[?2031l")
    expect(state.themeNotify).toBe(false)
  })

  it("finds the mode among others in one sequence, and leaves those to xterm", async () => {
    const { term, state, write } = rig()
    await write("\x1b[?1006;2031;2004h")
    expect(state.themeNotify).toBe(true)
    expect(term.modes.bracketedPasteMode).toBe(true)
  })

  it("stays as it was on any other private mode", async () => {
    const { state, write } = rig()
    await write("\x1b[?2004h")
    expect(state.themeNotify).toBe(false)
    state.themeNotify = true
    await write("\x1b[?2004l")
    expect(state.themeNotify).toBe(true)
  })

  it("answers DSR 996 with the scheme in force", async () => {
    const dark = rig("dark")
    await dark.write("\x1b[?996n")
    expect(dark.replies).toEqual(["\x1b[?997;1n"])
    const light = rig("light")
    await light.write("\x1b[?996n")
    expect(light.replies).toEqual(["\x1b[?997;2n"])
  })

  it("answers no other private DSR", async () => {
    const { replies, write } = rig()
    await write("\x1b[?15n")
    expect(replies).toEqual([])
  })

  it("stops listening once disposed", async () => {
    const { state, replies, watch, write } = rig()
    watch.dispose()
    await write("\x1b[?2031h\x1b[?996n")
    expect(state.themeNotify).toBe(false)
    expect(replies).toEqual([])
  })
})

describe("colorSchemeReport", () => {
  it("reports dark as 1 and light as 2", () => {
    expect(colorSchemeReport("dark")).toBe("\x1b[?997;1n")
    expect(colorSchemeReport("light")).toBe("\x1b[?997;2n")
  })
})
