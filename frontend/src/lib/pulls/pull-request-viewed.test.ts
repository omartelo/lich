import { describe, expect, it, vi } from "vitest"
import {
  fileFingerprint,
  isViewed,
  setViewed,
  subscribeViewed,
  viewedFiles,
} from "./pull-request-viewed"
import { parseDiff, type DiffFile } from "@/lib/git/diff"

// The store is module-level, so every test uses its own pull request URL.
const pr = (name: string) => `https://github.com/o/l/pull/${name}`

// A real parsed diff, so the fingerprint is exercised against the shape the
// screen actually hands it.
const diffOf = (body: string): DiffFile =>
  parseDiff(`diff --git a/a.ts b/a.ts\n--- a/a.ts\n+++ b/a.ts\n@@ -1,2 +1,2 @@\n${body}`)[0]

const original = diffOf(" keep\n-old\n+new\n")

describe("pull-request-viewed", () => {
  it("starts with nothing ticked", () => {
    expect(viewedFiles(pr("empty")).size).toBe(0)
  })

  it("ticks a file off and back on", () => {
    const url = pr("toggle")
    const fp = fileFingerprint(original)
    setViewed(url, "a.ts", fp, true)
    expect(isViewed(viewedFiles(url), "a.ts", fp)).toBe(true)
    setViewed(url, "a.ts", fp, false)
    expect(isViewed(viewedFiles(url), "a.ts", fp)).toBe(false)
  })

  it("keeps pull requests apart", () => {
    const fp = fileFingerprint(original)
    setViewed(pr("one"), "a.ts", fp, true)
    expect(isViewed(viewedFiles(pr("two")), "a.ts", fp)).toBe(false)
  })

  // The point of fingerprinting: a new commit unticks the files it rewrote and
  // leaves the rest of the review standing.
  it("unticks a file whose diff changed, and only that file", () => {
    const url = pr("rewritten")
    const rewritten = diffOf(" keep\n-old\n+newer\n")
    setViewed(url, "a.ts", fileFingerprint(original), true)
    setViewed(url, "b.ts", "b-unchanged", true)

    const ticks = viewedFiles(url)
    expect(isViewed(ticks, "a.ts", fileFingerprint(rewritten))).toBe(false)
    expect(isViewed(ticks, "b.ts", "b-unchanged")).toBe(true)
  })

  it("re-ticking at the new fingerprint sticks", () => {
    const url = pr("reticked")
    const rewritten = diffOf(" keep\n-old\n+newer\n")
    setViewed(url, "a.ts", fileFingerprint(original), true)
    setViewed(url, "a.ts", fileFingerprint(rewritten), true)
    expect(isViewed(viewedFiles(url), "a.ts", fileFingerprint(rewritten))).toBe(true)
  })

  // useSyncExternalStore re-renders forever if getSnapshot returns a fresh
  // object every call, so the same map must come back until something changes.
  it("returns a stable snapshot between changes", () => {
    const url = pr("stable")
    setViewed(url, "a.ts", "fp", true)
    const first = viewedFiles(url)
    expect(viewedFiles(url)).toBe(first)
    setViewed(url, "b.ts", "fp", true)
    expect(viewedFiles(url)).not.toBe(first)
    // The old snapshot is never mutated behind a holder's back.
    expect(first.has("b.ts")).toBe(false)
  })

  it("notifies subscribers, and only on a real change", () => {
    const url = pr("notify")
    const listener = vi.fn()
    const off = subscribeViewed(listener)
    setViewed(url, "a.ts", "fp", true)
    setViewed(url, "a.ts", "fp", true) // already ticked at this fingerprint
    setViewed(url, "b.ts", "fp", false) // never ticked
    expect(listener).toHaveBeenCalledTimes(1)
    off()
    setViewed(url, "c.ts", "fp", true)
    expect(listener).toHaveBeenCalledTimes(1)
  })
})

describe("fileFingerprint", () => {
  it("is stable for the same diff", () => {
    expect(fileFingerprint(original)).toBe(fileFingerprint(diffOf(" keep\n-old\n+new\n")))
  })

  it("changes when a line changes", () => {
    expect(fileFingerprint(original)).not.toBe(fileFingerprint(diffOf(" keep\n-old\n+newer\n")))
  })

  // The same lines under a different header mean the file's own change did not
  // move — an edit further up the file must not untick this one.
  it("ignores a hunk header shifted by an unrelated edit", () => {
    const moved = parseDiff(
      "diff --git a/a.ts b/a.ts\n--- a/a.ts\n+++ b/a.ts\n@@ -40,2 +41,2 @@\n keep\n-old\n+new\n",
    )[0]
    expect(fileFingerprint(moved)).toBe(fileFingerprint(original))
  })

  it("distinguishes a rename from an edit in place", () => {
    const renamed = parseDiff(
      "diff --git a/a.ts b/b.ts\nrename from a.ts\nrename to b.ts\n@@ -1,2 +1,2 @@\n keep\n-old\n+new\n",
    )[0]
    expect(fileFingerprint(renamed)).not.toBe(fileFingerprint(original))
  })

  it("fingerprints a binary file by its counts", () => {
    const binary = parseDiff(
      "diff --git a/i.png b/i.png\nBinary files a/i.png and b/i.png differ\n",
    )[0]
    expect(fileFingerprint(binary)).toBe(fileFingerprint(binary))
  })
})
