import { describe, expect, it } from "vitest"
import { decideProviderSetup } from "./provider-setup"
import type { ProviderState } from "./providers-store"

const p = (id: string, installed: boolean): ProviderState => ({
  id: id as ProviderState["id"],
  name: id,
  binary: id,
  installed,
  enabled: false,
  docs: `https://example.test/${id}`,
})

describe("decideProviderSetup", () => {
  it("waits while detection has not answered", () => {
    expect(decideProviderSetup([], "")).toEqual({ kind: "wait" })
    // Even with a stored default: an empty list is never a real machine state.
    expect(decideProviderSetup([], "codex")).toEqual({ kind: "wait" })
  })

  it("skips once a default has been chosen", () => {
    const list = [p("claude", true), p("codex", true)]
    expect(decideProviderSetup(list, "codex")).toEqual({ kind: "skip" })
  })

  it("preselects the first installed provider, not the first known one", () => {
    const list = [p("claude", false), p("codex", true), p("crush", true)]
    expect(decideProviderSetup(list, "")).toEqual({ kind: "show", preselect: "codex" })
  })

  it("shows with no preselection when nothing is installed", () => {
    const list = [p("claude", false), p("codex", false)]
    expect(decideProviderSetup(list, "")).toEqual({ kind: "show", preselect: null })
  })
})
