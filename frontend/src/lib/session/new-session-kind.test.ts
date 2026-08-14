import { describe, expect, it } from "vitest"
import { PROVIDER_KINDS } from "./sessions"
import { resolveNewSessionKind } from "./new-session-kind"

describe("resolveNewSessionKind", () => {
  it.each(PROVIDER_KINDS)("uses the %s project default for implicit sessions", (provider) => {
    expect(resolveNewSessionKind(undefined, provider)).toBe(provider)
  })

  it("preserves an explicit provider or terminal choice", () => {
    expect(resolveNewSessionKind("shell", "codex")).toBe("shell")
    expect(resolveNewSessionKind("omp", "codex")).toBe("omp")
  })
})
