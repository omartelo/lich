import { beforeEach, describe, expect, it, vi } from "vitest"

// The suite runs in node, which has no localStorage; the pref is stubbed so
// the round trip through it is what is checked.
const stored = new Map<string, string>()

vi.stubGlobal("localStorage", {
  getItem: (key: string) => stored.get(key) ?? null,
  setItem: (key: string, value: string) => {
    stored.set(key, value)
  },
  removeItem: (key: string) => {
    stored.delete(key)
  },
  key: (index: number) => [...stored.keys()][index] ?? null,
  get length() {
    return stored.size
  },
})

// The legacy carry-over runs once per module, so each test gets a fresh one.
async function freshStore() {
  vi.resetModules()
  return import("./panes-store")
}

beforeEach(() => {
  stored.clear()
})

const group = { id: "g1", name: "wall", cells: ["s1", "s2"], tracks: {} }

describe("storedGroups", () => {
  it("answers an identity-stable empty array when nothing is stored", async () => {
    const { storedGroups } = await freshStore()
    expect(storedGroups()).toEqual([])
    expect(storedGroups()).toBe(storedGroups())
  })

  it("keeps the parsed value while the stored string is unchanged", async () => {
    const { storedGroups, writeGroups } = await freshStore()
    writeGroups([group])
    const first = storedGroups()
    expect(first).toMatchObject([{ id: "g1", cells: ["s1", "s2"] }])
    expect(storedGroups()).toBe(first)
  })

  it("carries every project's walls of the old layout into the one list", async () => {
    stored.set("lich.panes.p1", JSON.stringify([group]))
    stored.set(
      "lich.panes.p2",
      JSON.stringify([{ id: "g2", name: "other", cells: ["s3", "s4"], tracks: {} }]),
    )
    const { storedGroups } = await freshStore()

    expect(storedGroups().map((wall) => wall.id)).toEqual(["g1", "g2"])
    expect(stored.has("lich.panes.p1")).toBe(false)
    expect(stored.has("lich.panes.p2")).toBe(false)
    expect(JSON.parse(stored.get("lich.panes") ?? "[]")).toHaveLength(2)
  })

  it("keeps walls already in the one list when old ones are carried in", async () => {
    stored.set("lich.panes", JSON.stringify([group]))
    stored.set(
      "lich.panes.p2",
      JSON.stringify([{ id: "g2", name: "other", cells: ["s3", "s4"], tracks: {} }]),
    )
    const { storedGroups } = await freshStore()

    expect(storedGroups().map((wall) => wall.id)).toEqual(["g1", "g2"])
  })
})

describe("writeGroups", () => {
  it("writes the whole list under the one key", async () => {
    const { writeGroups } = await freshStore()
    writeGroups([group])
    expect([...stored.keys()]).toEqual(["lich.panes"])
  })
})
