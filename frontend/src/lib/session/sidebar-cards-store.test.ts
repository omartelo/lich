import { describe, expect, it } from "vitest"
import { readSidebarCards, registerSidebarCards } from "./sidebar-cards-store"

describe("sidebar cards store", () => {
  it("reads the order the mounted sidebar registered, at the read", () => {
    let shown = true
    const unregister = registerSidebarCards(() => [{ id: "s1", shown }])
    shown = false
    expect(readSidebarCards()).toEqual([{ id: "s1", shown: false }])
    unregister()
    expect(readSidebarCards()).toEqual([])
  })

  // The rail mounts as the open sidebar unmounts: the outgoing cleanup must not
  // wipe the incoming registration.
  it("keeps a newer registration when an older one unregisters", () => {
    const unregisterOld = registerSidebarCards(() => [{ id: "old", shown: true }])
    const unregisterNew = registerSidebarCards(() => [{ id: "new", shown: true }])
    unregisterOld()
    expect(readSidebarCards()).toEqual([{ id: "new", shown: true }])
    unregisterNew()
  })
})
