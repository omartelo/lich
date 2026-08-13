import { describe, expect, it } from "vitest"
import { delegatePrompt, delegateWorktreePrompt } from "./delegate-prompt"

describe("delegatePrompt", () => {
  it("delegates in plain words where the sender has lich's tools", () => {
    // A colon, never a preposition: the task typed after it can be an order, a
    // question or pasted context, and none of them bends into "to <verb>".
    expect(delegatePrompt("claude", "docs")).toBe('Delegate to the "docs" session: ')
    expect(delegatePrompt("codex", "docs")).toBe('Delegate to the "docs" session: ')
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
    expect(delegatePrompt("claude", "fix login 2")).toBe('Delegate to the "fix login 2" session: ')
    expect(delegatePrompt("crush", "fix login 2")).toBe('lich send "fix login 2" "')
  })
})

describe("delegateWorktreePrompt", () => {
  it("stays plain prose where the sender has lich's tools", () => {
    expect(delegateWorktreePrompt("claude")).toBe("Delegate to a new worktree session: ")
    expect(delegateWorktreePrompt("codex")).toBe("Delegate to a new worktree session: ")
  })

  it("names both commands for every other sender — nothing else would", () => {
    for (const kind of ["opencode", "crush", "omp", "shell", "something-new"]) {
      const prompt = delegateWorktreePrompt(kind)
      expect(prompt).toContain("lich open --worktree")
      expect(prompt).toContain("lich send")
      expect(prompt.endsWith(": ")).toBe(true)
    }
  })
})
