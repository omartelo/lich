// @vitest-environment jsdom
import { EditorState } from "@codemirror/state"
import { EditorView } from "@codemirror/view"
import { afterEach, describe, expect, it } from "vitest"
import { type SlotElements, threadSlots } from "./codemirror-threads"

// The gap's element only exists once CodeMirror calls toDOM, so this suite
// mounts a real view under jsdom to reach the register callback and the
// microtask that announces it — the part codemirror-threads.test.ts, which
// runs in node, cannot.

const flush = () => new Promise<void>((resolve) => queueMicrotask(resolve))

let view: EditorView | null = null

afterEach(() => {
  view?.destroy()
  view = null
})

function mount(onChange: (elements: SlotElements) => void) {
  const slots = threadSlots(onChange)
  view = new EditorView({
    state: EditorState.create({ doc: "a\nbb\nccc", extensions: [slots.extension] }),
    parent: document.body,
  })
  return { slots, view }
}

describe("threadSlots", () => {
  it("hands React one element per gap, announced once for the whole batch", async () => {
    const seen: SlotElements[] = []
    const { slots, view } = mount((elements) => seen.push(elements))

    slots.update(view, [
      { key: "t:a", docLine: 1 },
      { key: "t:b", docLine: 2 },
    ])
    await flush()

    expect(seen).toHaveLength(1)
    expect([...seen[0].keys()].sort()).toEqual(["t:a", "t:b"])
    expect(seen[0].get("t:a")?.className).toBe("cm-thread-slot")
  })

  it("forgets a gap that is taken away, and announces that too", async () => {
    const seen: SlotElements[] = []
    const { slots, view } = mount((elements) => seen.push(elements))

    slots.update(view, [{ key: "t:a", docLine: 1 }])
    await flush()
    slots.update(view, [])
    await flush()

    expect(seen).toHaveLength(2)
    expect(seen[1].size).toBe(0)
  })

  it("keeps the element of a gap whose key survives an update", async () => {
    const seen: SlotElements[] = []
    const { slots, view } = mount((elements) => seen.push(elements))

    slots.update(view, [{ key: "t:a", docLine: 1 }])
    await flush()
    const before = seen[0].get("t:a")
    slots.update(view, [
      { key: "t:a", docLine: 1 },
      { key: "t:b", docLine: 3 },
    ])
    await flush()

    expect(seen[seen.length - 1].get("t:a")).toBe(before)
  })
})
