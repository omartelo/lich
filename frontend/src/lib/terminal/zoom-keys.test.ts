import { describe, expect, it } from "vitest"
import { defaultHotkeys, type KeyState } from "../hotkeys"
import { zoomIntent } from "./zoom-keys"

const press = (over: Partial<KeyState>): KeyState => ({
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  key: "",
  repeat: false,
  ...over,
})

describe("zoomIntent", () => {
  it("maps the default zoom chords, including a shifted Equal key", () => {
    const hotkeys = defaultHotkeys(false)
    expect(zoomIntent(press({ ctrlKey: true, shiftKey: true, key: "+" }), hotkeys, false)).toBe(
      "in",
    )
    expect(zoomIntent(press({ ctrlKey: true, key: "=" }), hotkeys, false)).toBe("in")
    expect(zoomIntent(press({ ctrlKey: true, key: "-" }), hotkeys, false)).toBe("out")
    expect(zoomIntent(press({ ctrlKey: true, key: "0" }), hotkeys, false)).toBe("reset")
  })

  it("uses Cmd rather than Ctrl for primary defaults on macOS", () => {
    const hotkeys = defaultHotkeys(false)
    expect(zoomIntent(press({ metaKey: true, key: "-" }), hotkeys, true)).toBe("out")
    expect(zoomIntent(press({ ctrlKey: true, key: "-" }), hotkeys, true)).toBeNull()
  })

  it("uses the current rebind and releases the old zoom chord", () => {
    const hotkeys = {
      ...defaultHotkeys(false),
      zoomIn: { mod: true, ctrl: false, shift: true, alt: false, key: "i" },
    }
    expect(zoomIntent(press({ ctrlKey: true, shiftKey: true, key: "i" }), hotkeys, false)).toBe(
      "in",
    )
    expect(zoomIntent(press({ ctrlKey: true, key: "+" }), hotkeys, false)).toBeNull()
  })

  it("returns no intent when a zoom action is unassigned", () => {
    const hotkeys = { ...defaultHotkeys(false), zoomReset: null }
    expect(zoomIntent(press({ ctrlKey: true, key: "0" }), hotkeys, false)).toBeNull()
  })
})
