import { describe, expect, it, vi } from "vitest"
import { installBrowserDefaults, isAppContextMenu, isBrowserChord } from "./browser-defaults"

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
    for (const key of ["T", "W", "N", "M", "P", "O", "Q", "Delete"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, shiftKey: true, key }))).toBe(true)
    }
  })

  it("leaves the devtools chords alone — Shift changes what the key means", () => {
    for (const key of ["i", "j", "c"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, shiftKey: true, key }))).toBe(false)
    }
  })

  it("leaves reload, editing chords and bare keys alone", () => {
    for (const key of ["r", "a", "c", "v", "x", "z", "k"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, key }))).toBe(false)
    }
    expect(isBrowserChord(chord({ key: "t" }))).toBe(false)
  })

  it("prevents browser find, zoom, navigation, and close accelerators", () => {
    for (const key of ["f", "+", "=", "-", "0", "[", "]", "q"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, key }))).toBe(true)
      expect(isBrowserChord(chord({ metaKey: true, key }))).toBe(true)
    }
    expect(isBrowserChord(chord({ ctrlKey: true, shiftKey: true, key: "+" }))).toBe(true)
    expect(isBrowserChord(chord({ altKey: true, key: "ArrowLeft" }))).toBe(true)
    expect(isBrowserChord(chord({ altKey: true, key: "ArrowRight" }))).toBe(true)
    expect(isBrowserChord(chord({ altKey: true, key: "F4" }))).toBe(true)
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

// The window is injected, so the wiring is checked without one: which phase each
// listener takes, and which events it is allowed to cancel. Both matter more than
// the matchers above — a keydown listener on the bubble phase reaches the browser
// too late, and Ctrl+W closes the window, which quits lich.
describe("installBrowserDefaults", () => {
  const install = () => {
    const listeners = new Map<string, { handler: (event: unknown) => void; capture: unknown }>()
    const target = {
      addEventListener: (type: string, handler: (event: unknown) => void, capture?: unknown) => {
        listeners.set(type, { handler, capture })
      },
    } as unknown as Window
    installBrowserDefaults(target)
    const fire = (type: string, event: Record<string, unknown>) => {
      const preventDefault = vi.fn()
      listeners.get(type)?.handler({ preventDefault, ...event })
      return preventDefault
    }
    return { listeners, fire }
  }

  it("takes the keydown in the capture phase, ahead of the browser", () => {
    expect(install().listeners.get("keydown")?.capture).toBe(true)
  })

  it("cancels a browser chord and leaves everything else through", () => {
    const { fire } = install()

    expect(
      fire("keydown", { ctrlKey: true, shiftKey: false, altKey: false, key: "w" }),
    ).toHaveBeenCalled()
    expect(
      fire("keydown", { ctrlKey: true, shiftKey: false, altKey: false, key: "r" }),
    ).not.toHaveBeenCalled()
  })

  it("prevents the browser default without stopping propagation to the TUI", () => {
    const { listeners } = install()
    const preventDefault = vi.fn()
    const stopPropagation = vi.fn()
    listeners.get("keydown")?.handler({
      ctrlKey: true,
      metaKey: false,
      shiftKey: false,
      altKey: false,
      key: "f",
      preventDefault,
      stopPropagation,
    })
    expect(preventDefault).toHaveBeenCalled()
    expect(stopPropagation).not.toHaveBeenCalled()
  })

  // The terminal keeps Chromium's menu — that is where its Copy and Paste live.
  it("cancels the context menu on the app's chrome only", () => {
    const { fire } = install()

    expect(fire("contextmenu", { target: { closest: () => null } })).toHaveBeenCalled()
    expect(fire("contextmenu", { target: { closest: () => ({}) } })).not.toHaveBeenCalled()
  })

  // Bubble phase, so a drop zone of our own runs first; unconditional, because
  // whatever is left has nowhere to land but over the app itself.
  it("cancels every unclaimed drag and drop, in the bubble phase", () => {
    const { listeners, fire } = install()

    expect(listeners.get("drop")?.capture).toBeUndefined()
    expect(listeners.get("dragover")?.capture).toBeUndefined()
    expect(fire("drop", {})).toHaveBeenCalled()
    expect(fire("dragover", {})).toHaveBeenCalled()
  })
})
