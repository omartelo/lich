import { afterEach, describe, expect, it, vi } from "vitest"
import { closePulls, isPullsOpen, openPulls, subscribePullsCard } from "./pulls-card-store"
import { isSettingsOpen } from "./settings-card-store"

// The store is module-level (shared across tests): every test closes what it
// opened so the next one starts from a clean slate.
afterEach(() => {
  closePulls("/repo")
  closePulls("/repo/.worktrees/feature")
})

describe("pulls-card-store", () => {
  it("parks and removes a checkout's card", () => {
    expect(isPullsOpen("/repo")).toBe(false)
    openPulls("/repo")
    expect(isPullsOpen("/repo")).toBe(true)
    closePulls("/repo")
    expect(isPullsOpen("/repo")).toBe(false)
  })

  it("tracks each worktree separately", () => {
    openPulls("/repo/.worktrees/feature")
    expect(isPullsOpen("/repo/.worktrees/feature")).toBe(true)
    expect(isPullsOpen("/repo")).toBe(false)
  })

  it("notifies subscribers on park and remove", () => {
    const listener = vi.fn()
    const off = subscribePullsCard(listener)
    openPulls("/repo")
    openPulls("/repo") // already parked
    closePulls("/repo")
    expect(listener).toHaveBeenCalledTimes(2)
    off()
  })

  // A pathless group (a project that is not a checkout) must not park a card.
  it("ignores an empty path", () => {
    openPulls("")
    expect(isPullsOpen("")).toBe(false)
  })

  // Both card stores come off the same factory; this proves they are wired to
  // separate instances and not to a shared set.
  it("is independent of the settings card store", () => {
    openPulls("/repo")
    expect(isSettingsOpen("/repo")).toBe(false)
  })
})
