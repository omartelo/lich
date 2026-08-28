import { describe, expect, it } from "vitest"
import { projectRoute, rememberProjectRoute } from "./project-route"

describe("projectRoute", () => {
  it("falls back to the project's own route until a screen is remembered", () => {
    expect(projectRoute("unseen")).toBe("/projects/unseen")
  })

  it("returns the screen the project was last on", () => {
    rememberProjectRoute("a", "/projects/a/settings")
    expect(projectRoute("a")).toBe("/projects/a/settings")
  })

  it("keeps one route per project, latest wins", () => {
    rememberProjectRoute("b", "/projects/b/settings")
    rememberProjectRoute("b", "/projects/b/pulls/all/12")
    expect(projectRoute("b")).toBe("/projects/b/pulls/all/12")
    expect(projectRoute("a")).toBe("/projects/a/settings")
  })
})
