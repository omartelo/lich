import { act } from "react"

// base-ui's default tooltip open delay (OPEN_DELAY in @base-ui/react/tooltip).
const OPEN_DELAY_MS = 600

// What a pointer resting on `target` reads in its Hint. The trigger of a reason
// hint is a <span>, which takes no focus, so this hovers rather than focusing.
export async function hoverHint(target: Element): Promise<string | null | undefined> {
  const events = [
    new PointerEvent("pointerenter", { pointerType: "mouse" }),
    new MouseEvent("mouseenter"),
    new MouseEvent("mousemove", { bubbles: true }),
  ]
  for (const event of events) {
    await act(async () => {
      target.dispatchEvent(event)
    })
  }
  await act(() => new Promise((resolve) => setTimeout(resolve, OPEN_DELAY_MS + 100)))
  return document.querySelector('[data-slot="tooltip-content"]')?.textContent
}
