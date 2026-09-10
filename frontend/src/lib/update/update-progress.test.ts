import { describe, expect, it } from "vitest"
import { downloadReading, failureText, isUpdateProgress } from "./update-progress"

describe("downloadReading", () => {
  it("reads percent and bytes off a known total", () => {
    expect(
      downloadReading({ phase: "download", received: 44_900_000, total: 118_200_000 }),
    ).toEqual({
      percent: 37,
      bytes: "44.9 MB of 118.2 MB",
    })
  })
  it("caps at 100 and floors, never rounding past what arrived", () => {
    expect(downloadReading({ phase: "download", received: 999, total: 1000 }).percent).toBe(99)
    expect(downloadReading({ phase: "download", received: 1200, total: 1000 }).percent).toBe(100)
  })
  it("is indeterminate without a total", () => {
    expect(downloadReading({ phase: "download", received: 5_000_000, total: -1 })).toEqual({
      percent: null,
      bytes: "5.0 MB",
    })
  })
})

describe("failureText", () => {
  it("names the download and how far it got", () => {
    expect(failureText({ phase: "download", received: 380, total: 1000 }, "connection reset")).toBe(
      "Download failed at 38%: connection reset",
    )
    expect(failureText({ phase: "download", received: 380, total: -1 }, "reset")).toBe(
      "Download failed: reset",
    )
  })
  it("names the install once the download is over", () => {
    expect(failureText({ phase: "install", received: 0, total: 0 }, "checksum mismatch")).toBe(
      "Install failed: checksum mismatch",
    )
    expect(failureText({ phase: "installer", received: 0, total: 0 }, "x")).toBe(
      "Install failed: x",
    )
  })
  it("says only that the update failed before any step arrived", () => {
    expect(failureText(null, "no release")).toBe("Update failed: no release")
  })
})

describe("isUpdateProgress", () => {
  it("accepts the three phases and rejects the rest", () => {
    expect(isUpdateProgress({ phase: "download", received: 1, total: 2 })).toBe(true)
    expect(isUpdateProgress({ phase: "installer", received: 0, total: 0 })).toBe(true)
    expect(isUpdateProgress({ phase: "verify", received: 0, total: 0 })).toBe(false)
    expect(isUpdateProgress({ phase: "download", received: "1", total: 2 })).toBe(false)
    expect(isUpdateProgress(null)).toBe(false)
  })
})
