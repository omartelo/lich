import { afterEach, describe, expect, it, vi } from "vitest"
import {
  closeSettings,
  isSettingsOpen,
  openSettings,
  subscribeSettingsCard,
} from "./settings-card-store"

// The store is module-level (shared across tests): every test closes what it
// opened so the next one starts from a clean slate.
afterEach(() => {
  closeSettings("a")
  closeSettings("b")
})

describe("settings-card-store", () => {
  it("starts closed", () => {
    expect(isSettingsOpen("a")).toBe(false)
  })

  it("open then close flips the flag", () => {
    openSettings("a")
    expect(isSettingsOpen("a")).toBe(true)
    closeSettings("a")
    expect(isSettingsOpen("a")).toBe(false)
  })

  it("tracks projects independently", () => {
    openSettings("a")
    expect(isSettingsOpen("a")).toBe(true)
    expect(isSettingsOpen("b")).toBe(false)
  })

  it("notifies subscribers on open and close", () => {
    const listener = vi.fn()
    const off = subscribeSettingsCard(listener)
    openSettings("a")
    closeSettings("a")
    expect(listener).toHaveBeenCalledTimes(2)
    off()
  })

  it("does not notify when the state is unchanged", () => {
    const listener = vi.fn()
    const off = subscribeSettingsCard(listener)
    openSettings("a")
    openSettings("a") // already open
    closeSettings("b") // never opened
    expect(listener).toHaveBeenCalledTimes(1)
    off()
  })

  it("stops notifying after unsubscribe", () => {
    const listener = vi.fn()
    const off = subscribeSettingsCard(listener)
    off()
    openSettings("a")
    expect(listener).not.toHaveBeenCalled()
  })
})
