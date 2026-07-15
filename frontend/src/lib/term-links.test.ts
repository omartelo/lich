import { afterEach, describe, expect, it, vi } from "vitest"
import type { ILink } from "ghostty-web"
import { System } from "./rpc"
import { openInBrowser } from "./term-links"

vi.mock("./rpc", () => ({ System: { OpenExternal: vi.fn() } }))

const openExternal = vi.mocked(System.OpenExternal)

function makeLink(text = "https://example.com"): ILink {
  return { text, range: {} as ILink["range"], activate: () => {} }
}

function click(overrides: Partial<MouseEvent> = {}): MouseEvent {
  return { ctrlKey: false, metaKey: false, ...overrides } as MouseEvent
}

afterEach(() => openExternal.mockClear())

describe("openInBrowser", () => {
  it("opens the URL in the OS browser on Ctrl-click", () => {
    openInBrowser(makeLink()).activate(click({ ctrlKey: true }))
    expect(openExternal).toHaveBeenCalledWith("https://example.com")
  })

  it("opens on Cmd-click too", () => {
    openInBrowser(makeLink("https://a.dev")).activate(click({ metaKey: true }))
    expect(openExternal).toHaveBeenCalledWith("https://a.dev")
  })

  it("does nothing on a plain click without a modifier", () => {
    openInBrowser(makeLink()).activate(click())
    expect(openExternal).not.toHaveBeenCalled()
  })

  it("preserves the detected link's text and range", () => {
    const link = makeLink("https://keep.me")
    const wrapped = openInBrowser(link)
    expect(wrapped.text).toBe(link.text)
    expect(wrapped.range).toBe(link.range)
  })
})
