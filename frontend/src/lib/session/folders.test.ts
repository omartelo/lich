import { describe, expect, it } from "vitest"
import {
  addSession,
  foldersOf,
  renameFolder,
  sessionsOf,
  setSessionsFolder,
  setSessionPinned,
  type SessionState,
} from "./sessions"
import {
  collapsedMark,
  FOLDER_KEY_PREFIX,
  folderKey,
  PINNED_GROUP_KEY,
  ROOT_GROUP_KEY,
  sidebarGroups,
} from "./sidebar-groups"

const P = "project-1"

function buildState(n: number): SessionState {
  let state: SessionState = addSession({}, P, "s1")
  for (let i = 2; i <= n; i++) {
    state = addSession(state, P, `s${i}`)
  }
  return state
}

const ids = (groups: ReturnType<typeof sidebarGroups>) =>
  groups.map((group) => [group.key, group.sessions.map((s) => s.id)])

describe("setSessionsFolder", () => {
  it("files a session and takes it back out", () => {
    let state = setSessionsFolder(buildState(2), P, ["s1"], "Apps")
    expect(sessionsOf(state, P)[0].folder).toBe("Apps")
    state = setSessionsFolder(state, P, ["s1"], "")
    expect(sessionsOf(state, P)[0].folder).toBeUndefined()
  })

  // The key is dropped rather than set to "": a session that has never been
  // filed and one taken out of a folder have to be the same shape, or a
  // comparison somewhere reads them as two different cards.
  it("leaves no folder key behind when a session is unfiled", () => {
    const filed = setSessionsFolder(buildState(1), P, ["s1"], "Apps")
    const state = setSessionsFolder(filed, P, ["s1"], "")
    expect("folder" in sessionsOf(state, P)[0]).toBe(false)
  })

  it("leaves the stored order alone", () => {
    const state = setSessionsFolder(buildState(3), P, ["s1"], "Apps")
    expect(sessionsOf(state, P).map((s) => s.id)).toEqual(["s1", "s2", "s3"])
  })

  it("ignores an unknown project or session", () => {
    const state = buildState(2)
    expect(setSessionsFolder(state, "nope", ["s1"], "Apps")).toBe(state)
    expect(setSessionsFolder(state, P, ["ghost"], "Apps")).toBe(state)
    expect(setSessionsFolder(state, P, [], "Apps")).toBe(state)
  })

  // A checkout's whole block is filed in one gesture, so the reducer takes the
  // list: one commit for the lot, not one repaint per card.
  it("files every session in the list at once", () => {
    const state = setSessionsFolder(buildState(4), P, ["s1", "s3"], "Apps")
    expect(sessionsOf(state, P).map((s) => s.folder)).toEqual([
      "Apps",
      undefined,
      "Apps",
      undefined,
    ])
  })

  it("takes a whole list back out of its folder", () => {
    let state = setSessionsFolder(buildState(3), P, ["s1", "s2"], "Apps")
    state = setSessionsFolder(state, P, ["s1", "s2"], "")
    expect(sessionsOf(state, P).every((s) => s.folder === undefined)).toBe(true)
  })

  // An id the project does not hold cannot make the write fail for the ones it
  // does: the block hands over what it drew, and a card closed mid-gesture is
  // exactly that.
  it("files the sessions it knows and ignores the rest", () => {
    const state = setSessionsFolder(buildState(2), P, ["s1", "ghost"], "Apps")
    expect(sessionsOf(state, P).map((s) => s.folder)).toEqual(["Apps", undefined])
  })
})

describe("renameFolder", () => {
  it("rewrites every session filed under the name", () => {
    let state = setSessionsFolder(buildState(3), P, ["s1"], "Apps")
    state = setSessionsFolder(state, P, ["s3"], "Apps")
    state = renameFolder(state, P, "Apps", "Applications")
    expect(sessionsOf(state, P).map((s) => s.folder)).toEqual([
      "Applications",
      undefined,
      "Applications",
    ])
  })

  it("ungroups the folder when the new name is empty", () => {
    let state = setSessionsFolder(buildState(2), P, ["s1"], "Apps")
    state = setSessionsFolder(state, P, ["s2"], "Apps")
    state = renameFolder(state, P, "Apps", "")
    expect(sessionsOf(state, P).every((s) => s.folder === undefined)).toBe(true)
  })

  // Without the guard this would sweep every unfiled session into a folder
  // nobody asked for — the same guard the store carries.
  it("refuses the empty source name", () => {
    const state = setSessionsFolder(buildState(2), P, ["s1"], "Apps")
    expect(renameFolder(state, P, "", "Everything")).toBe(state)
  })

  it("ignores a folder nothing is filed under", () => {
    const state = setSessionsFolder(buildState(2), P, ["s1"], "Apps")
    expect(renameFolder(state, P, "Infra", "Ops")).toBe(state)
  })
})

describe("foldersOf", () => {
  it("names each folder once, in the order its first card sits in", () => {
    let state = setSessionsFolder(buildState(4), P, ["s2"], "Apps")
    state = setSessionsFolder(state, P, ["s3"], "Design system")
    state = setSessionsFolder(state, P, ["s4"], "Apps")
    expect(foldersOf(state, P)).toEqual(["Apps", "Design system"])
  })

  it("answers nothing for a project with no folders, and for no project", () => {
    expect(foldersOf(buildState(2), P)).toEqual([])
    expect(foldersOf(buildState(2), "nope")).toEqual([])
  })
})

describe("sidebarGroups with folders", () => {
  it("draws a folder as its own block, above the checkout its cards came from", () => {
    let state = addSession({}, P, "s1")
    state = addSession(state, P, "wt1", "claude", "/wt/a")
    state = addSession(state, P, "wt2", "claude", "/wt/a")
    state = setSessionsFolder(state, P, ["wt1"], "Apps")
    expect(ids(sidebarGroups(sessionsOf(state, P)))).toEqual([
      [ROOT_GROUP_KEY, ["s1"]],
      [folderKey("Apps"), ["wt1"]],
      ["/wt/a", ["wt2"]],
    ])
  })

  // The whole point of filing by subject: a folder holds cards from checkouts
  // that have nothing else in common.
  it("gathers cards from every checkout into one folder", () => {
    let state = addSession({}, P, "s1")
    state = addSession(state, P, "wt1", "claude", "/wt/a")
    state = addSession(state, P, "wt2", "claude", "/wt/b")
    state = setSessionsFolder(state, P, ["s1"], "Apps")
    state = setSessionsFolder(state, P, ["wt1"], "Apps")
    state = setSessionsFolder(state, P, ["wt2"], "Apps")
    expect(ids(sidebarGroups(sessionsOf(state, P)))).toEqual([
      [folderKey("Apps"), ["s1", "wt1", "wt2"]],
    ])
  })

  // A folder named after a real checkout must not key the block that checkout
  // already keys, which is what the prefix is for.
  it("keys a folder apart from a worktree of the same name", () => {
    let state = addSession({}, P, "s1")
    state = addSession(state, P, "wt1", "claude", "/wt/a")
    state = setSessionsFolder(state, P, ["s1"], "/wt/a")
    const keys = sidebarGroups(sessionsOf(state, P)).map((group) => group.key)
    expect(keys).toEqual([`${FOLDER_KEY_PREFIX}/wt/a`, "/wt/a"])
    expect(new Set(keys).size).toBe(2)
  })

  // The pin promises the head of the list, not one particular header, so it
  // outranks the folder — the same rule a wall already outranks the pin by.
  it("draws a pinned card in the pinned block even when it is filed", () => {
    let state = setSessionsFolder(buildState(2), P, ["s1"], "Apps")
    state = setSessionPinned(state, P, "s1", true)
    expect(ids(sidebarGroups(sessionsOf(state, P)))).toEqual([
      [PINNED_GROUP_KEY, ["s1"]],
      [ROOT_GROUP_KEY, ["s2"]],
    ])
  })

  it("keeps the block where its first card sits in the stored list", () => {
    let state = buildState(4)
    state = setSessionsFolder(state, P, ["s3"], "Apps")
    state = setSessionsFolder(state, P, ["s4"], "Apps")
    expect(ids(sidebarGroups(sessionsOf(state, P)))).toEqual([
      [ROOT_GROUP_KEY, ["s1", "s2"]],
      [folderKey("Apps"), ["s3", "s4"]],
    ])
  })

  // A folder is the set of sessions carrying its name, so the last card leaving
  // is what ends it — there is no empty folder to draw.
  it("stops drawing a folder once its last card leaves", () => {
    let state = setSessionsFolder(buildState(2), P, ["s1"], "Apps")
    state = setSessionsFolder(state, P, ["s1"], "")
    expect(ids(sidebarGroups(sessionsOf(state, P)))).toEqual([[ROOT_GROUP_KEY, ["s1", "s2"]]])
  })

  it("carries the folder's name on the block it draws", () => {
    const state = setSessionsFolder(buildState(2), P, ["s2"], "Design system")
    const folder = sidebarGroups(sessionsOf(state, P)).find((group) => group.folder)
    expect(folder?.folder).toBe("Design system")
    expect(folder?.path).toBe("")
    expect(folder?.stage).toBeNull()
  })
})

describe("collapsedMark", () => {
  it("says nothing when no card in the block is in the queue", () => {
    expect(collapsedMark([{ id: "other", status: "waiting" }], ["s1", "s2"])).toBeNull()
    expect(collapsedMark([], ["s1"])).toBeNull()
  })

  it("reports an unread finished turn", () => {
    expect(collapsedMark([{ id: "s1", status: "done" }], ["s1"])).toBe("done")
  })

  // Amber outranks emerald for the reason it does on the card: the waiting
  // session is the one asking for something.
  it("lets a waiting session outrank a finished turn, in either order", () => {
    const waiting = { id: "s2", status: "waiting" } as const
    const done = { id: "s1", status: "done" } as const
    expect(collapsedMark([done, waiting], ["s1", "s2"])).toBe("wait")
    expect(collapsedMark([waiting, done], ["s1", "s2"])).toBe("wait")
  })
})
