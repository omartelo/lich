import { describe, expect, it } from "vitest"
import { peerMention, peerName } from "./peer-name"

describe("peerMention", () => {
  it("addresses the name and leaves room for what the user types next", () => {
    expect(peerMention("lich-4f2a")).toBe("@lich-4f2a ")
  })
})

describe("peerName", () => {
  it("names a session after its directory and its id tail", () => {
    expect(peerName("/home/me/code/lich", "4f2a1b3c-0000-4000-8000-000000000000")).toBe("lich-4f2a")
  })

  it("separates two sessions sharing a checkout", () => {
    const cwd = "/home/me/code/lich"
    const first = peerName(cwd, "4f2a1b3c-0000-4000-8000-000000000000")
    const second = peerName(cwd, "9d8e7f6a-0000-4000-8000-000000000000")
    expect(first).not.toBe(second)
  })

  it("ignores a trailing separator", () => {
    expect(peerName("/home/me/code/lich/", "4f2a1b3c")).toBe("lich-4f2a")
  })

  it("reads a Windows path", () => {
    expect(peerName("C:\\Users\\me\\code\\lich", "4f2a1b3c")).toBe("lich-4f2a")
  })

  it("falls back when there is no directory to name", () => {
    expect(peerName("", "4f2a1b3c")).toBe("lich-4f2a")
    expect(peerName("/", "4f2a1b3c")).toBe("lich-4f2a")
  })

  it("keeps the directory alone when the id carries nothing usable", () => {
    expect(peerName("/home/me/code/lich", "---")).toBe("lich")
  })
})
