// @vitest-environment jsdom
import { act, createElement } from "react"
import { createRoot, type Root } from "react-dom/client"
import { afterEach, beforeEach, expect, test } from "vitest"
import { CommentBox } from "./CommentBox"

let host: HTMLDivElement
let root: Root

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  host = document.createElement("div")
  document.body.append(host)
  root = createRoot(host)
})

afterEach(() => {
  act(() => root.unmount())
  host.remove()
})

// A diff refetch rebuilds the editor under the box and mounts it again with the
// text already typed; the reviewer carries on from where they stopped.
test("a box mounted with text puts the caret after it", () => {
  const body = "rename this to something clearer"
  act(() =>
    root.render(
      createElement(CommentBox, {
        value: body,
        onChange: () => {},
        onSubmit: () => {},
        submitLabel: "Add to batch",
        autoFocus: true,
      }),
    ),
  )
  const field = host.querySelector("textarea") as HTMLTextAreaElement
  expect(document.activeElement).toBe(field)
  expect(field.selectionStart).toBe(body.length)
  expect(field.selectionEnd).toBe(body.length)
})
