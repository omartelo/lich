import { afterEach, expect, test, vi } from "vitest"
import { parseFooterVisibility, readFooterVisibility } from "./footer-prefs"

afterEach(() => vi.unstubAllGlobals())

test.each([true, false])("preserves the legacy model visibility %s", (context) => {
  expect(parseFooterVisibility(null, context)).toEqual({
    model: context,
    plan: true,
    handsOn: true,
    clock: false,
  })
})

test.each(["{", "null", "[]", "true", '"off"', '{"model":"false","plan":0}'])(
  "invalid preferences %s retain safe defaults",
  (raw) => {
    expect(parseFooterVisibility(raw, false)).toEqual({
      model: false,
      plan: true,
      handsOn: true,
      clock: false,
    })
  },
)

test("valid choices override only their own defaults", () => {
  expect(
    parseFooterVisibility('{"model":true,"plan":false,"clock":true,"future":false}', false),
  ).toEqual({ model: true, plan: false, handsOn: true, clock: true })
})

// Contract changed: layout owns new writes; these choices are read for migration.
test("reads legacy display choices independently of context and provider", () => {
  const stored = new Map<string, string>()
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => stored.get(key) ?? null,
    setItem: (key: string, value: string) => stored.set(key, value),
  })
  const visibility = { model: false, plan: false, handsOn: false, clock: true }
  stored.set("lich.footer.visibility", JSON.stringify(visibility))
  expect(readFooterVisibility(true)).toEqual(visibility)
  expect(readFooterVisibility(false)).toEqual(visibility)
  expect(stored.size).toBe(1)
})
