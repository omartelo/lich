// @vitest-environment jsdom
import { act, createElement } from "react"
import { createRoot, type Root } from "react-dom/client"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { DEFAULT_HOTKEYS } from "@/lib/hotkeys"
import { runHotkey, useHotkey } from "./use-hotkey"

vi.mock("@/providers/settings", () => ({
  useSettings: () => ({ hotkeys: DEFAULT_HOTKEYS }),
}))

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function Dock({ onToggle }: { onToggle: () => void }) {
  useHotkey("toggleDock", onToggle)
  return null
}

let root: Root

beforeEach(() => {
  root = createRoot(document.createElement("div"))
})

afterEach(() => {
  act(() => root.unmount())
})

const mount = (onToggle: () => void) => act(() => root.render(createElement(Dock, { onToggle })))

describe("runHotkey", () => {
  it("runs the handler the mounted action answers its chord with", () => {
    const handler = vi.fn()
    mount(handler)
    expect(runHotkey("toggleDock")).toBe(true)
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it("reports an action that declined", () => {
    mount(() => false)
    expect(runHotkey("toggleDock")).toBe(false)
  })

  it("reports an action nothing mounted handles", () => {
    mount(vi.fn())
    act(() => root.render(null))
    expect(runHotkey("toggleDock")).toBe(false)
  })

  it("runs the latest handler a re-render passed", () => {
    const first = vi.fn()
    const second = vi.fn()
    mount(first)
    mount(second)
    runHotkey("toggleDock")
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledTimes(1)
  })
})

describe("useHotkey", () => {
  it("still answers the chord bound to the action", () => {
    const handler = vi.fn()
    mount(handler)
    const event = new KeyboardEvent("keydown", {
      key: "D",
      ctrlKey: true,
      shiftKey: true,
      cancelable: true,
      bubbles: true,
    })
    document.body.dispatchEvent(event)
    expect(handler).toHaveBeenCalledTimes(1)
    expect(event.defaultPrevented).toBe(true)
  })
})
