// @vitest-environment jsdom
//
// A stored theme lich cannot load used to vanish from Appearance with only a
// log line. The list is what keeps it on screen, with its reason and a way out.
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test, vi } from "vitest"
import type { BrokenTheme } from "@/lib/api-types"
import { BrokenThemeList } from "./BrokenThemeList"

const FUTURE: BrokenTheme = {
  id: "tokyo-night",
  reason: "theme format 2 is newer than this lich reads (up to 1); update lich",
}

test("names each broken theme with its reason and removes it by id", async () => {
  const onRemove = vi.fn()
  const mounted = await mountBudget(
    createElement(BrokenThemeList, {
      themes: [FUTURE, { id: "gruvbox", reason: 'unknown app color "x"' }],
      onRemove,
    }),
  )

  expect(document.body.textContent).toContain("2 themes can't load")
  expect(document.body.textContent).toContain(FUTURE.reason)
  const remove = document.querySelector<HTMLButtonElement>('[aria-label="Remove tokyo-night"]')
  await mounted.act(() => remove?.click())
  expect(onRemove).toHaveBeenCalledWith(FUTURE)
  await mounted.unmount()
})

test("draws nothing when every theme loads", async () => {
  const mounted = await mountBudget(
    createElement(BrokenThemeList, { themes: [], onRemove: vi.fn() }),
  )
  expect(document.querySelector('[aria-label="Themes that can\'t load"]')).toBeNull()
  await mounted.unmount()
})
