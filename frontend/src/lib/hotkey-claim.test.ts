import { describe, expect, it, vi } from "vitest"
import { claimHotkey, DEFAULT_HOTKEYS, matchesCombo, type Combo, type KeyState } from "./hotkeys"
import { zoomIntent } from "./terminal/zoom-keys"

const press = (over: Partial<KeyState & { code: string }> = {}): KeyState & { code: string } => ({
  ctrlKey: true,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  key: "k",
  code: "KeyK",
  repeat: false,
  ...over,
})

describe("claimHotkey", () => {
  it("runs exactly one of two conflicting global actions", () => {
    const event = press()
    const first = vi.fn()
    const second = vi.fn()
    const matched = matchesCombo(event, DEFAULT_HOTKEYS.commandPalette, false)

    expect(claimHotkey(event, matched, first)).toBe(true)
    expect(claimHotkey(event, matched, second)).toBe(false)
    expect(first).toHaveBeenCalledOnce()
    expect(second).not.toHaveBeenCalled()
  })

  it("lets a global rebind claim a physical zoom chord first", () => {
    const event = press({ key: "-", code: "Minus" })
    const globalAction = vi.fn()
    const zoomAction = vi.fn()
    const binding: Combo = { mod: true, ctrl: false, shift: false, alt: false, key: "-" }

    expect(claimHotkey(event, matchesCombo(event, binding, false), globalAction)).toBe(true)
    expect(claimHotkey(event, zoomIntent(event) !== null, zoomAction)).toBe(false)
    expect(globalAction).toHaveBeenCalledOnce()
    expect(zoomAction).not.toHaveBeenCalled()
  })

  it("lets physical zoom claim a conflicting global rebind first", () => {
    const event = press({ key: "-", code: "Minus" })
    const globalAction = vi.fn()
    const zoomAction = vi.fn()
    const binding: Combo = { mod: true, ctrl: false, shift: false, alt: false, key: "-" }

    expect(claimHotkey(event, zoomIntent(event) !== null, zoomAction)).toBe(true)
    expect(claimHotkey(event, matchesCombo(event, binding, false), globalAction)).toBe(false)
    expect(zoomAction).toHaveBeenCalledOnce()
    expect(globalAction).not.toHaveBeenCalled()
  })

  it("lets the next claimant run after a handler declines", () => {
    const event = press()
    const declined = vi.fn(() => false as const)
    const accepted = vi.fn()

    expect(claimHotkey(event, true, declined)).toBe(false)
    expect(claimHotkey(event, true, accepted)).toBe(true)
    expect(declined).toHaveBeenCalledOnce()
    expect(accepted).toHaveBeenCalledOnce()
  })

  it("does not claim a disabled binding", () => {
    const event = press()
    const action = vi.fn()

    expect(claimHotkey(event, matchesCombo(event, null, false), action)).toBe(false)
    expect(action).not.toHaveBeenCalled()
  })
})
