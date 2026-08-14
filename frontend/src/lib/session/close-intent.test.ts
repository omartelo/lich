import { describe, expect, it } from "vitest"
import { closeIntent } from "./close-intent"
import type { Session } from "./sessions"

const session = (over: Partial<Session> = {}): Session => ({
  id: "s1",
  label: "Session 1",
  kind: "claude",
  ...over,
})

describe("closeIntent", () => {
  it("closes a plain idle session outright", () => {
    const s = session()
    expect(closeIntent(s, [s], false)).toBe("close")
  })

  it("asks before killing a running turn", () => {
    const s = session()
    expect(closeIntent(s, [s], true)).toBe("confirm-running")
  })

  it("asks about the turn before the worktree, never both at once", () => {
    const s = session({ path: "/wt/feature" })
    expect(closeIntent(s, [s], true)).toBe("confirm-running")
    // What the caller asks again once that has been confirmed.
    expect(closeIntent(s, [s], false)).toBe("ask-worktree")
  })

  it("leaves a worktree with another session alone", () => {
    const s = session({ path: "/wt/feature" })
    const sibling = session({ id: "s2", path: "/wt/feature" })
    expect(closeIntent(s, [s, sibling], false)).toBe("close")
  })

  it("refuses a pinned session whatever else is true", () => {
    const s = session({ pinned: true, path: "/wt/feature" })
    expect(closeIntent(s, [s], true)).toBe("refuse")
    expect(closeIntent(s, [s], false)).toBe("refuse")
  })
})
