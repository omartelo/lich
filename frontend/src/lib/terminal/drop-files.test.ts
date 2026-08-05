import { describe, expect, it } from "vitest"
import { composeDroppedPaths } from "./drop-files"

const PASTE_START = "\x1b[200~"
const PASTE_END = "\x1b[201~"

describe("composeDroppedPaths", () => {
  it("writes nothing for an empty drop", () => {
    expect(composeDroppedPaths([])).toBe("")
  })

  // Bracketed, so the prompt takes the block as one paste and leaves it unsent
  // — the user presses Enter, as after any drop into a terminal.
  it("wraps the paths as a single unsent paste", () => {
    expect(composeDroppedPaths(["/home/u/a.ts"])).toBe(`${PASTE_START}/home/u/a.ts ${PASTE_END}`)
  })

  it("separates a batch and leaves room for what is typed next", () => {
    const paste = composeDroppedPaths(["/home/u/a.ts", "/home/u/b.ts"])

    expect(paste).toBe(`${PASTE_START}/home/u/a.ts /home/u/b.ts ${PASTE_END}`)
  })

  // A path with a space is one argument or it is two files that do not exist.
  it("quotes a path a shell would split", () => {
    expect(composeDroppedPaths(["/home/u/my notes.md"])).toBe(
      `${PASTE_START}'/home/u/my notes.md' ${PASTE_END}`,
    )
  })

  it("quotes a path holding a quote", () => {
    expect(composeDroppedPaths(["/home/u/it's.md"])).toBe(
      `${PASTE_START}'/home/u/it'\\''s.md' ${PASTE_END}`,
    )
  })

  // Backslash and the drive colon are in the safe set, so an ordinary path of
  // either OS is written bare.
  it("leaves a plain Windows path alone", () => {
    expect(composeDroppedPaths(["C:\\Users\\u\\a.ts", "/home/u/a.ts"], true)).toBe(
      `${PASTE_START}C:\\Users\\u\\a.ts /home/u/a.ts ${PASTE_END}`,
    )
  })

  // cmd.exe has no single quotes: quoting a Windows path with them hands the
  // prompt a path that does not exist, quotes and all.
  it("quotes a Windows path with a space the way cmd.exe reads it", () => {
    expect(composeDroppedPaths(["C:\\Program Files\\a.ts"], true)).toBe(
      `${PASTE_START}"C:\\Program Files\\a.ts" ${PASTE_END}`,
    )
  })
})
