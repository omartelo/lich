import { describe, expect, it } from "vitest"
import { isAppContextMenu, isBrowserChord } from "./browser-defaults"

const chord = (over: Partial<KeyboardEvent>) =>
  ({
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    altKey: false,
    key: "a",
    ...over,
  }) as KeyboardEvent

describe("isBrowserChord", () => {
  it("swallows the tab, window, file and page commands under either modifier", () => {
    for (const key of ["t", "w", "n", "p", "s", "o", "u", "d", "h", "j"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, key }))).toBe(true)
      expect(isBrowserChord(chord({ metaKey: true, key }))).toBe(true)
    }
  })

  it("swallows the shifted commands, including the browsing-data wipe", () => {
    for (const key of ["T", "W", "N", "M", "Delete"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, shiftKey: true, key }))).toBe(true)
    }
  })

  it("leaves the devtools chords alone — Shift changes what the key means", () => {
    for (const key of ["i", "j", "c"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, shiftKey: true, key }))).toBe(false)
    }
  })

  it("leaves reload, editing chords and bare keys alone", () => {
    for (const key of ["r", "a", "c", "v", "x", "z", "f", "k"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, key }))).toBe(false)
    }
    expect(isBrowserChord(chord({ key: "t" }))).toBe(false)
  })

  it("ignores AltGr, which arrives as Ctrl+Alt and types real characters", () => {
    expect(isBrowserChord(chord({ ctrlKey: true, altKey: true, key: "w" }))).toBe(false)
  })
})

describe("isAppContextMenu", () => {
  const target = (closestHit: boolean) =>
    ({ closest: () => (closestHit ? {} : null) }) as unknown as EventTarget

  it("claims the app's own chrome", () => {
    expect(isAppContextMenu(target(false))).toBe(true)
  })

  it("leaves a terminal's menu alone — that is where Copy and Paste live", () => {
    expect(isAppContextMenu(target(true))).toBe(false)
  })

  it("claims a null target rather than letting the browser menu through", () => {
    expect(isAppContextMenu(null)).toBe(true)
  })
})
