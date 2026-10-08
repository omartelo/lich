import { afterEach, describe, expect, it, vi } from "vitest"
import { blobUrl, formatBlobSize, previewKind, readBlob } from "./preview"

describe("previewKind", () => {
  it.each([
    ["docs/shot.png", "image"],
    ["a/B.JPG", "image"],
    ["icon.webp", "image"],
    ["anim.gif", "image"],
    ["docs/architecture.pdf", "pdf"],
    ["Report.PDF", "pdf"],
  ])("%s previews as %s", (rel, kind) => {
    expect(previewKind(rel)).toBe(kind)
  })

  it.each([
    "logo.svg",
    "photo.heic",
    "demo.mp4",
    "src/png",
    ".png",
    "assets.png/readme",
    "archive.tar.gz",
  ])("%s has no preview", (rel) => {
    expect(previewKind(rel)).toBeNull()
  })
})

describe("blobUrl", () => {
  it("carries the token and escapes every part", () => {
    const url = new URL(
      blobUrl(
        { base: "http://127.0.0.1:9000", token: "t&k" },
        "/home/me/repo x",
        "a b/c#.png",
        "HEAD",
      ),
    )
    expect(url.pathname).toBe("/blob")
    expect(Object.fromEntries(url.searchParams)).toEqual({
      token: "t&k",
      path: "/home/me/repo x",
      rel: "a b/c#.png",
      ref: "HEAD",
    })
  })
})

describe("readBlob", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function stubBackend(response: Response) {
    vi.stubGlobal("window", { location: { href: "http://127.0.0.1:9000/?token=tok" } })
    const fetchMock = vi.fn().mockResolvedValue(response)
    vi.stubGlobal("fetch", fetchMock)
    return fetchMock
  }

  it("turns the bytes into an object URL", async () => {
    stubBackend(new Response(new Uint8Array([1, 2, 3]), { status: 200 }))
    const created = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:x")
    expect(await readBlob("/repo", "a.png", "")).toEqual({ state: "ok", url: "blob:x", bytes: 3 })
    created.mockRestore()
  })

  it("reads a 404 as the side with no file", async () => {
    stubBackend(new Response("a.png at HEAD: file does not exist\n", { status: 404 }))
    expect(await readBlob("/repo", "a.png", "HEAD")).toEqual({ state: "missing" })
  })

  it("carries the backend's words for a refusal", async () => {
    stubBackend(new Response("34.2 MB, above the 20 MB preview limit\n", { status: 413 }))
    expect(await readBlob("/repo", "a.png", "")).toEqual({
      state: "refused",
      message: "34.2 MB, above the 20 MB preview limit",
    })
  })
})

describe("formatBlobSize", () => {
  it.each([
    [0, "0.0 KB"],
    [39_322, "38.4 KB"],
    [1024 * 1024, "1.0 MB"],
    [1_258_291, "1.2 MB"],
  ])("%d bytes reads %s", (bytes, text) => {
    expect(formatBlobSize(bytes)).toBe(text)
  })
})
