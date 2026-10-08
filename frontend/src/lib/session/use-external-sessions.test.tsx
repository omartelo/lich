// @vitest-environment jsdom
//
// The external list is the costly one: a walk of every provider's store and a
// git call per project. What is pinned is when it is asked for: once per
// enabling, never while disabled, and a late answer never lands after the tab
// was left.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { createElement, useLayoutEffect, useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { ExternalSession } from "@/lib/api-types"
import { useExternalSessions } from "@/lib/session/use-external-sessions"

let calls = 0
let answer: (rows: ExternalSession[] | null) => void = () => {}

vi.mock("@/lib/rpc", () => ({
  Store: {
    ExternalSessions: () => {
      calls++
      return new Promise<ExternalSession[] | null>((resolve) => {
        answer = resolve
      })
    },
  },
}))

const row: ExternalSession = {
  kind: "claude",
  providerSessionId: "c1",
  title: "fix login",
  path: "",
  projectId: "p1",
  projectName: "alpha",
  projectPath: "/src/alpha",
  updatedAt: 1,
}

// The newest frame, which is the one the palette would be painting.
function last<T>(items: T[]): T | undefined {
  return items[items.length - 1]
}

// Probe renders the hook with `enabled` held in state, handing the setter out
// so a test can leave and re-enter the tab the way the palette does.
function probe(frames: ExternalSession[][], toggles: ((on: boolean) => void)[], initial: boolean) {
  return function Probe() {
    const [enabled, setEnabled] = useState(initial)
    toggles.push(setEnabled)
    const rows = useExternalSessions(enabled)
    useLayoutEffect(() => {
      frames.push(rows)
    })
    return null
  }
}

const mount = async (initial: boolean) => {
  const frames: ExternalSession[][] = []
  const toggles: ((on: boolean) => void)[] = []
  const mounted = await mountBudget(createElement(probe(frames, toggles, initial)))
  const toggle = (on: boolean) => mounted.act(() => last(toggles)?.(on))
  return { frames, mounted, toggle }
}

describe("useExternalSessions", () => {
  beforeEach(() => {
    calls = 0
  })

  it("asks nothing while disabled", async () => {
    const { frames, mounted } = await mount(false)
    expect(calls).toBe(0)
    expect(last(frames)).toEqual([])
    await mounted.unmount()
  })

  it("asks once per enabling and shows the answer", async () => {
    const { frames, mounted, toggle } = await mount(true)
    expect(calls).toBe(1)
    await mounted.act(() => answer([row]))
    expect(last(frames)).toEqual([row])

    await toggle(false)
    expect(last(frames)).toEqual([])
    await toggle(true)
    expect(calls).toBe(2)
    await mounted.unmount()
  })

  it("treats a null answer as none", async () => {
    const { frames, mounted } = await mount(true)
    await mounted.act(() => answer(null))
    expect(last(frames)).toEqual([])
    await mounted.unmount()
  })

  it("drops an answer that arrives after the tab was left", async () => {
    const { frames, mounted, toggle } = await mount(true)
    const late = answer
    await toggle(false)
    await mounted.act(() => late([row]))
    expect(last(frames)).toEqual([])
    await mounted.unmount()
  })
})
