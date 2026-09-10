import { afterEach, expect, test, vi } from "vitest"
import {
  DEFAULT_FOOTER_LAYOUT,
  FOOTER_ITEMS,
  footerZone,
  hasFooterItem,
  moveFooterItem,
  parseFooterLayout,
  readFooterLayout,
  resolveFooterLayout,
  writeFooterLayout,
  type FooterLayout,
} from "./footer-layout"

afterEach(() => vi.unstubAllGlobals())
const layout: FooterLayout = {
  left: ["attach", "files", "changes"],
  right: ["model", "context", "plan"],
}

test.each([null, "{", "null", "[]", '{"left":[]}', '{"left":[],"right":false}'])(
  "rejects malformed layout %s",
  (raw) => {
    expect(parseFooterLayout(raw)).toBeNull()
  },
)
test("preserves empty sides, drops unknown items and deduplicates across sides", () => {
  expect(parseFooterLayout('{"left":[],"right":[]}')).toEqual({ left: [], right: [] })
  expect(
    parseFooterLayout('{"left":["model","future",2,"model"],"right":["model","cost","files"]}'),
  ).toEqual({ left: ["model"], right: ["cost", "files"] })
})
test("migrates old visibility, context and cost without restoring hidden indicators", () => {
  expect(
    resolveFooterLayout(
      null,
      { model: false, plan: true, handsOn: false, clock: true },
      false,
      true,
    ),
  ).toEqual({ left: DEFAULT_FOOTER_LAYOUT.left, right: ["checkout", "plan", "cost", "clock"] })
  expect(
    resolveFooterLayout(
      layout,
      { model: false, plan: false, handsOn: false, clock: true },
      false,
      false,
    ),
  ).toBe(layout)
})
test("removed Settings item is discarded from either saved side and cannot be added", () => {
  expect(parseFooterLayout('{"left":["settings","attach"],"right":["model","settings"]}')).toEqual({
    left: ["attach"],
    right: ["model"],
  })
  expect(moveFooterItem(layout, "settings", "left")).toBe(layout)
})
test.each([
  ["attach", "changes", { left: ["files", "changes", "attach"], right: layout.right }],
  ["context", "model", { left: layout.left, right: ["context", "model", "plan"] }],
  ["model", "files", { left: ["attach", "model", "files", "changes"], right: ["context", "plan"] }],
  ["model", "left", { left: ["attach", "files", "changes", "model"], right: ["context", "plan"] }],
  ["clock", "right", { left: layout.left, right: ["model", "context", "plan", "clock"] }],
  ["model", "available", { left: layout.left, right: ["context", "plan"] }],
  ["model", "clock", { left: layout.left, right: ["context", "plan"] }],
])("moves %s onto %s without changing the source layout", (id, target, expected) => {
  const before = JSON.stringify(layout)
  expect(moveFooterItem(layout, id, target)).toEqual(expected)
  expect(JSON.stringify(layout)).toBe(before)
})
test("empty sides accept a first item and invalid/cancelled moves preserve identity", () => {
  expect(moveFooterItem({ left: [], right: [] }, "model", "left")).toEqual({
    left: ["model"],
    right: [],
  })
  expect(moveFooterItem(layout, "unknown", "left")).toBe(layout)
  expect(moveFooterItem(layout, "model", "outside")).toBe(layout)
  expect(moveFooterItem(layout, "model", "model")).toBe(layout)
  expect(moveFooterItem(layout, "clock", "available")).toBe(layout)
})
test("each item can move between both sides and available without duplication", () => {
  for (const item of FOOTER_ITEMS) {
    let next = moveFooterItem(layout, item.id, "left")
    expect(footerZone(next, item.id)).toBe("left")
    next = moveFooterItem(next, item.id, "right")
    expect(footerZone(next, item.id)).toBe("right")
    expect([...next.left, ...next.right].filter((id) => id === item.id)).toHaveLength(1)
    next = moveFooterItem(next, item.id, "available")
    expect(hasFooterItem(next, item.id)).toBe(false)
  }
})
test("persists both sides and their exact order", () => {
  const stored = new Map<string, string>()
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => stored.get(key) ?? null,
    setItem: (key: string, value: string) => stored.set(key, value),
  })
  expect(readFooterLayout()).toBeNull()
  writeFooterLayout(layout)
  expect(readFooterLayout()).toEqual(layout)
})

test("the backend cost choice wins over a layout saved by another browser profile", () => {
  const visibility = { model: true, plan: true, handsOn: true, clock: false }
  expect(
    resolveFooterLayout({ left: ["cost", "model"], right: [] }, visibility, true, false),
  ).toEqual({ left: ["model"], right: [] })
  expect(resolveFooterLayout({ left: ["model"], right: [] }, visibility, true, true)).toEqual({
    left: ["model"],
    right: ["cost"],
  })
})
