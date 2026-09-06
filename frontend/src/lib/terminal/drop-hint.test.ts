import { expect, it } from "vitest"
import { dropHintDetail } from "./drop-hint"

it("says what the drop does, and the copy for a confined session", () => {
  expect(dropHintDetail(false)).toBe("Its path is pasted at the prompt")
  expect(dropHintDetail(true)).toBe("Outside the checkout it arrives as a copy")
})
