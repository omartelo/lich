import { describe, expect, it } from "vitest"
import { queueFork, takeFork } from "./fork-queue"

describe("fork queue", () => {
  it("hands the queued parent conversation to the first spawn", () => {
    queueFork("s1", "conv-parent")
    expect(takeFork("s1")).toBe("conv-parent")
  })

  // One-shot: the second spawn of that card — a restart, a respawn after a
  // reload — must open its own conversation, not branch the parent again.
  it("is spent after one take", () => {
    queueFork("s2", "conv-parent")
    takeFork("s2")
    expect(takeFork("s2")).toBe("")
  })

  it("answers empty for a session nobody forked", () => {
    expect(takeFork("never-queued")).toBe("")
  })

  it("keeps one mark per session", () => {
    queueFork("s3", "conv-a")
    queueFork("s4", "conv-b")
    expect(takeFork("s4")).toBe("conv-b")
    expect(takeFork("s3")).toBe("conv-a")
  })
})
