import { renderToStaticMarkup } from "react-dom/server"
import { expect, test } from "vitest"
import { Input } from "./input"

// HTML attribute names are case-insensitive and React spells some in camelCase
// on the server, so the markup is compared lowercased.
const markup = (element: JSX.Element) => renderToStaticMarkup(element).toLowerCase()

// lich's fields take paths, branch names, filters and commands: a red squiggle
// under every one of them and a browser history dropdown are both noise.
test("an input opts out of spellcheck and autocomplete by default", () => {
  const html = markup(<Input />)
  expect(html).toContain('spellcheck="false"')
  expect(html).toContain('autocomplete="off"')
})

test("a caller can still turn either back on", () => {
  const html = markup(<Input spellCheck autoComplete="on" />)
  expect(html).toContain('spellcheck="true"')
  expect(html).toContain('autocomplete="on"')
})
