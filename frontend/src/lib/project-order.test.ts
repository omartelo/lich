import { describe, expect, it } from "vitest"
import type { Project } from "./api-types"
import { neighborProjectId } from "./project-order"

const project = (id: string): Project => ({ id, name: id, path: `/tmp/${id}` }) as Project

// Home is stored last here on purpose: it is appended when it is missing, and the
// tab strip still draws it first.
const projects = [project("a"), project("b"), project("home")]

describe("neighborProjectId", () => {
  it("walks the strip's order, Home first", () => {
    expect(neighborProjectId(projects, "home", "home", 1)).toBe("a")
    expect(neighborProjectId(projects, "home", "a", 1)).toBe("b")
  })

  it("wraps at both ends", () => {
    expect(neighborProjectId(projects, "home", "b", 1)).toBe("home")
    expect(neighborProjectId(projects, "home", "home", -1)).toBe("b")
  })

  it("has nowhere to go with a single tab", () => {
    expect(neighborProjectId([project("home")], "home", "home", 1)).toBe("")
    expect(neighborProjectId([], "home", undefined, 1)).toBe("")
  })

  it("lands on the end the step comes from when nothing is active", () => {
    expect(neighborProjectId(projects, "home", undefined, 1)).toBe("home")
    expect(neighborProjectId(projects, "home", undefined, -1)).toBe("b")
  })

  it("keeps the stored order when there is no Home", () => {
    expect(neighborProjectId(projects, null, "b", 1)).toBe("home")
    expect(neighborProjectId(projects, null, "a", -1)).toBe("home")
  })
})
