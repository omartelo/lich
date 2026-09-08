import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import {
  composeDroppedPaths,
  type DroppedFile,
  readDroppedFiles,
  resolveDroppedFiles,
} from "./drop-files"

// The backend is the boundary: the lookup is an RPC and the upload its own
// endpoint, so both are stubbed and the test asserts which branch each entry
// took, which is the whole point of the notices.
const resolve = vi.fn()
vi.mock("@/lib/rpc", () => ({
  DropService: {
    Resolve: (root: string, items: unknown[], confined: boolean) => resolve(root, items, confined),
  },
  endpoint: () => ({ base: "http://127.0.0.1:47821", token: "t" }),
}))

// The session a drop landed on. Confined is the interesting half — the backend
// searches one tree fewer for it, and the copy is what reaches the agent.
const target = (confined = false) => ({ cwd: "/home/u", sessionId: "s1", confined })

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

  // The notice is what tells a copy's path from the user's own, and it belongs
  // in the prompt rather than in a toast: the agent reads the prompt. The
  // trailing space gives way to a newline, so what is typed next lands under
  // the line instead of on it.
  it("writes the copy's line under the path, in the same paste", () => {
    const paste = composeDroppedPaths(["/cfg/dropped/b.png"], false, ["[lich] copy of b.png; …"])

    expect(paste).toBe(`${PASTE_START}/cfg/dropped/b.png\n[lich] copy of b.png; …\n${PASTE_END}`)
  })

  it("keeps one line per copy in a mixed drop", () => {
    const paste = composeDroppedPaths(["/home/u/a.ts", "/cfg/dropped/b.png"], false, ["copy of b"])

    expect(paste).toBe(`${PASTE_START}/home/u/a.ts /cfg/dropped/b.png\ncopy of b\n${PASTE_END}`)
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

  // PowerShell is the shell a Windows session runs, and it doubles an embedded
  // single quote rather than escaping it the POSIX way — a path under
  // C:\\Users\\O'Brien has to come out as one argument on both hosts.
  it("quotes a Windows path the way PowerShell reads it", () => {
    expect(composeDroppedPaths(["C:\\Program Files\\a.ts"], true)).toBe(
      `${PASTE_START}'C:\\Program Files\\a.ts' ${PASTE_END}`,
    )
    expect(composeDroppedPaths(["C:\\Users\\O'Brien\\a b.ts"], true)).toBe(
      `${PASTE_START}'C:\\Users\\O''Brien\\a b.ts' ${PASTE_END}`,
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
      json: async () => ({ path: "/cfg/lich/dropped/b.png", notice: "[lich] copy of b.png; …" }),
    })
    vi.stubGlobal("fetch", upload)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("reports no copy for a file the tree holds", async () => {
    resolve.mockResolvedValue(["/home/u/a.ts"])

    const result = await resolveDroppedFiles(target(), [file("a.ts")])

    expect(result).toEqual({ paths: ["/home/u/a.ts"], skipped: [], notices: [] })
    expect(upload).not.toHaveBeenCalled()
  })

  // The path pasted for b.png is a copy's, and the backend's line is what says
  // so at the prompt; the file found in the tree brings none.
  it("carries the backend's line for the entries pasted as a copy", async () => {
    resolve.mockResolvedValue(["/home/u/a.ts", ""])

    const result = await resolveDroppedFiles(target(), [file("a.ts"), file("b.png")])

    expect(result).toEqual({
      paths: ["/home/u/a.ts", "/cfg/lich/dropped/b.png"],
      skipped: [],
      notices: ["[lich] copy of b.png; …"],
    })
  })

  // A copy that was never written has no path to warn about.
  it("leaves a failed upload out of the copies", async () => {
    resolve.mockResolvedValue([""])
    upload.mockResolvedValue({ ok: false, status: 500 })

    const result = await resolveDroppedFiles(target(), [file("b.png")])

    expect(result).toEqual({ paths: [], skipped: ["b.png"], notices: [] })
  })

  // A directory has no bytes to copy, so it is skipped, never listed as one.
  it("does not call a missing folder a copy", async () => {
    resolve.mockResolvedValue([""])
    const folder: DroppedFile = { name: "docs", size: 0, mtime: 1, dir: true, blob: null }

    const result = await resolveDroppedFiles(target(), [folder])

    expect(result.notices).toEqual([])
    expect(result.skipped).toHaveLength(1)
    expect(upload).not.toHaveBeenCalled()
  })

  // Bigger than internal/drop's maxUpload: refused here rather than sent for the
  // backend to refuse, and the reason names the ceiling.
  it("skips a file too big to upload instead of sending it", async () => {
    resolve.mockResolvedValue([""])
    const huge: DroppedFile = {
      name: "core.dump",
      size: 33 * 1024 * 1024,
      mtime: 1,
      dir: false,
      blob: { size: 33 * 1024 * 1024 } as Blob,
    }

    const result = await resolveDroppedFiles(target(), [huge])

    expect(result.paths).toEqual([])
    expect(result.notices).toEqual([])
    expect(result.skipped).toEqual(["core.dump (over 32MB)"])
    expect(upload).not.toHaveBeenCalled()
  })

  // The upload is keyed by session: that is the directory a confined session
  // reads the copy through, and the one its close deletes.
  it("uploads a copy under the session it was dropped on", async () => {
    resolve.mockResolvedValue([""])

    await resolveDroppedFiles(target(), [file("b.png")])

    const url = String(upload.mock.calls[0][0])
    expect(url).toContain("session=s1")
    expect(url).toContain("name=b.png")
  })

  // A confined session's home holds nothing it can open, so the backend is told
  // not to search it — the copy is the only path that reaches the agent.
  it("tells the backend a confined session searches one tree fewer", async () => {
    resolve.mockResolvedValue([""])

    const result = await resolveDroppedFiles(target(true), [file("b.png")])

    expect(resolve).toHaveBeenCalledWith("/home/u", expect.anything(), true)
    expect(result.notices).toEqual(["[lich] copy of b.png; …"])
  })

  // A folder is the one drop with no answer: no path, and no copy to make of a
  // tree, so the refusal has to be said out loud rather than left as a drop
  // that did nothing. Same sentence confined or not: what the session can reach
  // is its checkout either way.
  it.each([false, true])("says a folder cannot be handed over (confined: %s)", async (confined) => {
    resolve.mockResolvedValue([""])
    const folder: DroppedFile = { name: "docs", size: 0, mtime: 1, dir: true, blob: null }

    const result = await resolveDroppedFiles(target(confined), [folder])

    expect(result.skipped).toEqual([
      "docs (folders outside the checkout cannot be handed over; drop files)",
    ])
    expect(result.paths).toEqual([])
  })

  it("answers an empty drop without touching the backend", async () => {
    expect(await resolveDroppedFiles(target(), [])).toEqual({
      paths: [],
      skipped: [],
      notices: [],
    })
    expect(resolve).not.toHaveBeenCalled()
  })
})

// DataTransfer is the browser's, but every field readDroppedFiles takes off it
// is read synchronously through a plain interface — which is what lets the drop
// be described here without a DOM.
describe("readDroppedFiles", () => {
  const item = (file: unknown, dir?: boolean): DataTransferItem =>
    ({
      kind: file === null ? "string" : "file",
      getAsFile: () => file,
      webkitGetAsEntry: dir === undefined ? undefined : () => ({ isDirectory: dir }),
    }) as unknown as DataTransferItem

  const transfer = (items: DataTransferItem[]): DataTransfer =>
    ({ items }) as unknown as DataTransfer

  it("reads what the page can say about a dropped file", () => {
    const dropped = readDroppedFiles(
      transfer([item({ name: "a.ts", size: 12, lastModified: 1699, type: "" }, false)]),
    )

    expect(dropped).toHaveLength(1)
    expect(dropped[0]).toMatchObject({ name: "a.ts", size: 12, mtime: 1699, dir: false })
    expect(dropped[0].blob).not.toBeNull()
  })

  // A folder has no bytes for the page to upload, and its size is not the
  // backend's to match on — the lookup is by name and mtime alone.
  it("marks a directory, zeroes its size and carries no blob", () => {
    const dropped = readDroppedFiles(
      transfer([item({ name: "docs", size: 4096, lastModified: 42, type: "" }, true)]),
    )

    expect(dropped).toEqual([{ name: "docs", size: 0, mtime: 42, dir: true, blob: null }])
  })

  // Text dragged out of the app's own UI, and an item the browser hands over
  // without a file behind it: neither is a drop.
  it("skips a non-file item and a file that resolves to nothing", () => {
    expect(readDroppedFiles(transfer([item(null), item(undefined, false)]))).toEqual([])
  })

  // webkitGetAsEntry is the only carrier of the directory flag and it is not
  // guaranteed: without it the entry is read as a file rather than dropped.
  it("reads an entry as a file when the browser offers no entry API", () => {
    const dropped = readDroppedFiles(
      transfer([item({ name: "a.ts", size: 3, lastModified: 7, type: "" })]),
    )

    expect(dropped[0]).toMatchObject({ name: "a.ts", size: 3, dir: false })
  })
})
