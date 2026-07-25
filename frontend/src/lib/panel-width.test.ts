import { describe, expect, it } from "vitest"
import { clampRem, dragWidth, parseStoredWidth } from "./panel-width"

const bounds = { minRem: 12, maxRem: 30 }

describe("clampRem", () => {
  it("passes values inside the bounds through", () => {
    expect(clampRem(20, bounds)).toBe(20)
  })

  it("clamps to min and max", () => {
    expect(clampRem(5, bounds)).toBe(12)
    expect(clampRem(99, bounds)).toBe(30)
  })
})

describe("parseStoredWidth", () => {
  it("reads a valid stored width, clamped", () => {
    expect(parseStoredWidth("18", bounds, 15)).toBe(18)
    expect(parseStoredWidth("99", bounds, 15)).toBe(30)
  })

  it.each([null, "", "garbage", "-3", "0", "NaN"])("falls back to the default for %j", (raw) => {
    expect(parseStoredWidth(raw, bounds, 15)).toBe(15)
  })
})

describe("dragWidth", () => {
  it("grows a right-edge panel when dragging right", () => {
    expect(dragWidth(15, 100, 132, "right", bounds)).toBe(17)
  })

  it("shrinks a right-edge panel when dragging left", () => {
    expect(dragWidth(15, 100, 68, "right", bounds)).toBe(13)
  })

  it("grows a left-edge panel when dragging left", () => {
    expect(dragWidth(15, 100, 68, "left", bounds)).toBe(17)
  })

  it("clamps the dragged width", () => {
    expect(dragWidth(15, 100, 1000, "right", bounds)).toBe(30)
    expect(dragWidth(15, 100, -1000, "right", bounds)).toBe(12)
  })
})
