// @vitest-environment jsdom
//
// jsdom for the window the store follows the pointer on; it has no layout, so
// the hit test's elementFromPoint is stubbed per test.
import type { ClientRect, CollisionDetection, Modifier, UniqueIdentifier } from "@dnd-kit/core"
import { afterEach, describe, expect, it, vi } from "vitest"
import {
  currentFileDrag,
  dropState,
  endFileDrag,
  FILE_TARGET_ATTRIBUTE,
  type FileDrag,
  fileAwareCollision,
  hangBelowTarget,
  setFileDropOver,
  startFileDrag,
  subscribeFileDrag,
} from "./file-drag-store"

const drag = (of: Partial<FileDrag> = {}): FileDrag => ({ folder: "", path: "", over: null, ...of })

afterEach(() => {
  endFileDrag()
  vi.restoreAllMocks()
})

describe("dropState", () => {
  it("draws nothing while no card is dragged", () => {
    expect(dropState({ folder: "Apps", path: "" }, null)).toBe("idle")
    expect(dropState(null, null)).toBe("idle")
  })

  it("dims a block nothing is filed into", () => {
    expect(dropState(null, drag())).toBe("refuses")
  })

  it("takes a card into any folder but its own", () => {
    const apps = { folder: "Apps", path: "" }
    expect(dropState(apps, drag())).toBe("accepts")
    expect(dropState(apps, drag({ folder: "Design" }))).toBe("accepts")
    expect(dropState(apps, drag({ folder: "Apps" }))).toBe("refuses")
  })

  it("lets a checkout take back only a filed card of its own", () => {
    const worktree = { folder: "", path: "/wt/front" }
    expect(dropState(worktree, drag({ folder: "Apps", path: "/wt/front" }))).toBe("accepts")
    expect(dropState(worktree, drag({ path: "/wt/front" }))).toBe("refuses")
    expect(dropState(worktree, drag({ folder: "Apps", path: "/wt/back" }))).toBe("refuses")
    expect(dropState({ folder: "", path: "" }, drag({ folder: "Apps" }))).toBe("accepts")
  })

  it("marks the target under the pointer, and only that one", () => {
    expect(dropState({ folder: "Apps", path: "" }, drag({ over: "Apps" }))).toBe("over")
    expect(dropState({ folder: "Design", path: "" }, drag({ over: "Apps" }))).toBe("accepts")
    const root = { folder: "", path: "" }
    expect(dropState(root, drag({ folder: "Apps", over: "" }))).toBe("over")
  })
})

describe("the drag in flight", () => {
  it("notifies when the target under the pointer changes, not when it stays", () => {
    const listener = vi.fn()
    const unsubscribe = subscribeFileDrag(listener)
    startFileDrag("Apps", "/wt/front")
    expect(currentFileDrag()).toEqual({ folder: "Apps", path: "/wt/front", over: null })

    setFileDropOver("Design")
    setFileDropOver("Design")
    expect(currentFileDrag()?.over).toBe("Design")
    expect(listener).toHaveBeenCalledTimes(2)

    endFileDrag()
    endFileDrag()
    expect(currentFileDrag()).toBeNull()
    expect(listener).toHaveBeenCalledTimes(3)
    unsubscribe()
  })

  it("ignores a target reported with no drag in flight", () => {
    setFileDropOver("Apps")
    expect(currentFileDrag()).toBeNull()
  })

  it("follows the pointer onto a header wearing the target attribute, until the drag ends", () => {
    const header = document.createElement("div")
    header.setAttribute(FILE_TARGET_ATTRIBUTE, "Apps")
    const title = document.createElement("span")
    header.append(title)
    document.elementFromPoint = vi.fn(() => title)

    startFileDrag("", "")
    window.dispatchEvent(new MouseEvent("pointermove", { clientX: 10, clientY: 20 }))
    expect(document.elementFromPoint).toHaveBeenCalledWith(10, 20)
    expect(currentFileDrag()?.over).toBe("Apps")

    document.elementFromPoint = vi.fn(() => document.body)
    window.dispatchEvent(new MouseEvent("pointermove", { clientX: 10, clientY: 200 }))
    expect(currentFileDrag()?.over).toBeNull()

    endFileDrag()
    window.dispatchEvent(new MouseEvent("pointermove", { clientX: 10, clientY: 20 }))
    expect(document.elementFromPoint).toHaveBeenCalledTimes(1)
  })
})

describe("fileAwareCollision", () => {
  const rect: ClientRect = { top: 0, left: 0, bottom: 10, right: 10, width: 10, height: 10 }
  const args = {
    active: { id: "s1" },
    collisionRect: rect,
    droppableRects: new Map<UniqueIdentifier, ClientRect>([["s2", rect]]),
    droppableContainers: [{ id: "s2" }],
    pointerCoordinates: { x: 5, y: 5 },
  } as unknown as Parameters<CollisionDetection>[0]

  it("is closestCenter off every header", () => {
    startFileDrag("", "")
    expect(fileAwareCollision(args).map((collision) => collision.id)).toEqual(["s2"])
  })

  it("is over the dragged card alone while a header takes the drop", () => {
    startFileDrag("Apps", "")
    setFileDropOver("")
    expect(fileAwareCollision(args)).toEqual([{ id: "s1" }])
  })
})

// Carried by its middle, the card would sit on the header it is dropped on and
// hide the highlight saying it will land there; over a target it hangs under
// the pointer instead, and anywhere else it moves exactly as the drag says.
describe("hangBelowTarget", () => {
  const rect = { top: 300, left: 0, bottom: 386, right: 240, width: 240, height: 86 }
  const at = (activatorEvent: Event) =>
    ({
      activatorEvent,
      draggingNodeRect: rect,
      transform: { x: 0, y: -200, scaleX: 1, scaleY: 1 },
    }) as unknown as Parameters<Modifier>[0]
  // Pressed at y 340, 40px into the card, then carried 200px up to y 140.
  const press = new MouseEvent("pointerdown", { clientY: 340 })

  it("leaves the card where the drag put it off every header", () => {
    startFileDrag("", "")
    expect(hangBelowTarget(at(press)).y).toBe(-200)
  })

  it("hangs the card's top just under the pointer over a header", () => {
    startFileDrag("", "")
    setFileDropOver("Apps")
    expect(rect.top + hangBelowTarget(at(press)).y).toBe(140 + 8)
  })

  it("leaves a keyboard drag alone, which has no pointer to hang from", () => {
    startFileDrag("", "")
    setFileDropOver("Apps")
    expect(hangBelowTarget(at(new KeyboardEvent("keydown"))).y).toBe(-200)
  })
})
