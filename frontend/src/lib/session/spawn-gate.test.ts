import { describe, expect, it } from "vitest"
import { CHECKOUT_GONE, CONVERSATION_GONE, spawnDecision, type SpawnProbe } from "./spawn-gate"
import type { Session } from "./sessions"

const resumable: Session = {
  id: "s1",
  label: "session",
  kind: "claude",
  providerSessionId: "conv-uuid",
}

function probe(over: Partial<SpawnProbe> = {}): SpawnProbe {
  return {
    workdirMissing: () => Promise.resolve(false),
    resumeAvailable: () => Promise.resolve(true),
    ...over,
  }
}

describe("spawnDecision", () => {
  it("spawns when there is nothing to resume", async () => {
    expect(await spawnDecision("/repo", null, probe())).toEqual({ verdict: "spawn" })
  })

  it("asks when the conversation is still there", async () => {
    expect(await spawnDecision("/repo", resumable, probe())).toEqual({ verdict: "ask" })
  })

  it("starts fresh when the conversation is gone", async () => {
    const decision = await spawnDecision(
      "/repo",
      resumable,
      probe({ resumeAvailable: () => Promise.resolve(false) }),
    )
    expect(decision).toEqual({ verdict: "fresh", notice: CONVERSATION_GONE })
  })

  // Crush keeps one conversation database per checkout, so the directory is
  // part of the question: asked without it, every restored Crush card would be
  // told its conversation is gone.
  it("asks about the conversation in the session's own directory", async () => {
    const asked: string[] = []
    await spawnDecision(
      "/repo/worktrees/feature",
      { ...resumable, kind: "crush" },
      probe({
        resumeAvailable: (_kind, _id, cwd) => {
          asked.push(cwd)
          return Promise.resolve(true)
        },
      }),
    )
    expect(asked).toEqual(["/repo/worktrees/feature"])
  })

  it("parks when the checkout is gone, without asking about the conversation", async () => {
    let asked = false
    const decision = await spawnDecision(
      "/gone",
      resumable,
      probe({
        workdirMissing: () => Promise.resolve(true),
        resumeAvailable: () => {
          asked = true
          return Promise.resolve(true)
        },
      }),
    )
    expect(decision).toEqual({ verdict: "park", notice: CHECKOUT_GONE })
    expect(asked).toBe(false)
  })

  // Both checks fail safe, in opposite directions: never close a session on an
  // answer that never came, never drop a conversation that may still be live.
  it("spawns when the workdir check fails", async () => {
    const decision = await spawnDecision(
      "/repo",
      null,
      probe({ workdirMissing: () => Promise.reject(new Error("rpc down")) }),
    )
    expect(decision).toEqual({ verdict: "spawn" })
  })

  it("still asks when the resume check fails", async () => {
    const decision = await spawnDecision(
      "/repo",
      resumable,
      probe({ resumeAvailable: () => Promise.reject(new Error("rpc down")) }),
    )
    expect(decision).toEqual({ verdict: "ask" })
  })
})
