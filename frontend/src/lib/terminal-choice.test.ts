import {describe, expect, it} from "vitest"
import {TERMINAL_CHOICE_KEY, useGhosttyTerminal} from "./terminal-choice"

function storageWith(value: string | null): Pick<Storage, "getItem"> {
  return {getItem: (key: string) => (key === TERMINAL_CHOICE_KEY ? value : null)}
}

describe("useGhosttyTerminal", () => {
  it("is xterm by default and ghostty only on the exact opt-out", () => {
    expect(useGhosttyTerminal(storageWith(null))).toBe(false)
    expect(useGhosttyTerminal(storageWith("xterm"))).toBe(false)
    expect(useGhosttyTerminal(storageWith("ghostty"))).toBe(true)
  })

  it("is xterm without a storage (node/test environment)", () => {
    expect(useGhosttyTerminal(undefined)).toBe(false)
    expect(useGhosttyTerminal(null)).toBe(false)
  })
})
