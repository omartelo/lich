import { describe, expect, it } from "vitest"
import { delegatePrompt } from "./delegate-prompt"

describe("delegatePrompt", () => {
  it("asks in plain words where the sender has lich's tools", () => {
    expect(delegatePrompt("claude", "docs")).toBe('Ask the "docs" session to ')
    expect(delegatePrompt("codex", "docs")).toBe('Ask the "docs" session to ')
  })

  it("spells the command for every other sender", () => {
    for (const kind of ["opencode", "crush", "omp", "shell"]) {
      expect(delegatePrompt(kind, "docs")).toBe('lich send "docs" "')
    }
  })

  it("treats a kind it does not know as having no tools", () => {
    // A provider from a newer build: the command works everywhere, so guessing
    // it has tools is the only guess that can leave the agent stuck.
    expect(delegatePrompt("something-new", "docs")).toBe('lich send "docs" "')
  })

  it("carries the label through verbatim", () => {
    expect(delegatePrompt("claude", "fix login 2")).toBe('Ask the "fix login 2" session to ')
    expect(delegatePrompt("crush", "fix login 2")).toBe('lich send "fix login 2" "')
  })
})
