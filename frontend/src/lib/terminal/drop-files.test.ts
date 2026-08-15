import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { composeDroppedPaths, type DroppedFile, resolveDroppedFiles } from "./drop-files"

// The backend is the boundary: the lookup is an RPC and the upload its own
// endpoint, so both are stubbed and the test asserts which branch each entry
// took — the whole point of the copied list.
const resolve = vi.fn()
vi.mock("@/lib/rpc", () => ({
  DropService: {
    Resolve: (root: string, items: unknown[]) => resolve(root, items),
  },
  endpoint: () => ({ base: "http://127.0.0.1:47821", token: "t" }),
}))

const file = (name: string): DroppedFile => ({
  name,
  size: 3,
  mtime: 1,
  dir: false,
  blob: new Blob(["abc"]),
})

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

describe("resolveDroppedFiles", () => {
  const upload = vi.fn()

  beforeEach(() => {
    resolve.mockReset()
    upload.mockReset()
    upload.mockResolvedValue({
      ok: true,
      json: async () => ({ path: "/cfg/lich/dropped/b.png" }),
    })
    vi.stubGlobal("fetch", upload)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("reports no copy for a file the tree holds", async () => {
    resolve.mockResolvedValue(["/home/u/a.ts"])

    const result = await resolveDroppedFiles("/home/u", [file("a.ts")])

    expect(result).toEqual({ paths: ["/home/u/a.ts"], skipped: [], copied: [] })
    expect(upload).not.toHaveBeenCalled()
  })

  // The path pasted for b.png is a copy's, and only this list says so.
  it("names the entries pasted as a copy", async () => {
    resolve.mockResolvedValue(["/home/u/a.ts", ""])

    const result = await resolveDroppedFiles("/home/u", [file("a.ts"), file("b.png")])

    expect(result).toEqual({
      paths: ["/home/u/a.ts", "/cfg/lich/dropped/b.png"],
      skipped: [],
      copied: ["b.png"],
    })
  })

  // A copy that was never written has no path to warn about.
  it("leaves a failed upload out of the copies", async () => {
    resolve.mockResolvedValue([""])
    upload.mockResolvedValue({ ok: false, status: 500 })

    const result = await resolveDroppedFiles("/home/u", [file("b.png")])

    expect(result).toEqual({ paths: [], skipped: ["b.png"], copied: [] })
  })

  // A directory has no bytes to copy, so it is skipped, never listed as one.
  it("does not call a missing folder a copy", async () => {
    resolve.mockResolvedValue([""])
    const folder: DroppedFile = { name: "docs", size: 0, mtime: 1, dir: true, blob: null }

    const result = await resolveDroppedFiles("/home/u", [folder])

    expect(result.copied).toEqual([])
    expect(result.skipped).toHaveLength(1)
    expect(upload).not.toHaveBeenCalled()
  })

  it("answers an empty drop without touching the backend", async () => {
    expect(await resolveDroppedFiles("/home/u", [])).toEqual({
      paths: [],
      skipped: [],
      copied: [],
    })
    expect(resolve).not.toHaveBeenCalled()
  })
})
