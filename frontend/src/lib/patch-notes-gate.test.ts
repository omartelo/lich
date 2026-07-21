import {describe, expect, it} from "vitest"
import {decidePatchNotes} from "./patch-notes-gate"
import type {PatchNotes} from "./api-types"

const notes: PatchNotes = {
  version: "0.11.0",
  groups: [{label: "Added", items: ["**A thing.** It does stuff."]}],
}

describe("decidePatchNotes", () => {
  it("shows when the seen version differs from the current one", () => {
    expect(decidePatchNotes(notes, "0.10.0")).toEqual({
      kind: "show",
      version: "0.11.0",
      notes,
    })
  })

  it("records silently on the first run (nothing seen yet), so launch is quiet", () => {
    expect(decidePatchNotes(notes, null)).toEqual({kind: "record", version: "0.11.0"})
  })

  it("does nothing when the current version was already seen", () => {
    expect(decidePatchNotes(notes, "0.11.0")).toEqual({kind: "none"})
  })

  it("does nothing without a section — a dev build or unreleased version", () => {
    expect(decidePatchNotes({version: "dev", groups: null}, "0.10.0")).toEqual({kind: "none"})
    expect(decidePatchNotes({version: "0.11.0", groups: []}, "0.10.0")).toEqual({kind: "none"})
    expect(decidePatchNotes({version: "", groups: null}, null)).toEqual({kind: "none"})
  })
})
