import { describe, expect, it } from "vitest"
import { pullRequestHandoff } from "./pr-handoff"
import type { CheckItem, PullRequestDetail } from "@/lib/api-types"

const detail = (over: Partial<PullRequestDetail> = {}): PullRequestDetail => ({
  number: 130,
  url: "https://github.com/o/l/pull/130",
  title: "feat: something",
  body: "",
  author: "omartelo",
  state: "OPEN",
  isDraft: false,
  mergeable: "MERGEABLE",
  mergeStateStatus: "CLEAN",
  reviewDecision: "",
  reviewers: null,
  baseRefName: "main",
  headRefName: "quiet-willow",
  changedFiles: 1,
  isCrossRepository: false,
  maintainerCanModify: false,
  checks: { total: 0, passed: 0, failed: 0, pending: 0 },
  checkRuns: null,
  commits: null,
  ...over,
})

const run = (name: string, state: CheckItem["state"] = "failed"): CheckItem => ({
  name,
  state,
  description: "",
  url: `https://github.com/o/l/runs/${name}`,
  startedAt: "",
  completedAt: "",
})

const red = (names: string[]): Partial<PullRequestDetail> => ({
  checks: { total: names.length, passed: 0, failed: names.length, pending: 0 },
  checkRuns: names.map((name) => run(name)),
})

describe("pullRequestHandoff", () => {
  it("has nothing to hand over on a clean pull request", () => {
    expect(pullRequestHandoff(detail())).toBeNull()
  })

  it("says nothing about a pull request that is over", () => {
    expect(pullRequestHandoff(detail({ state: "MERGED", ...red(["build"]) }))).toBeNull()
  })

  it("names the conflict, from either field", () => {
    for (const over of [{ mergeStateStatus: "DIRTY" }, { mergeable: "CONFLICTING" }]) {
      const action = pullRequestHandoff(detail(over))
      expect(action?.label).toBe("Resolve conflicts")
      expect(action?.prompt).toContain("#130 (quiet-willow) has merge conflicts with main")
    }
  })

  it("puts the conflict ahead of red CI", () => {
    const action = pullRequestHandoff(detail({ mergeStateStatus: "DIRTY", ...red(["build"]) }))
    expect(action?.label).toBe("Resolve conflicts")
  })

  it("names the failed checks and their runs", () => {
    const action = pullRequestHandoff(detail(red(["build", "test"])))
    expect(action?.label).toBe("Fix CI errors")
    expect(action?.prompt).toContain("- build — https://github.com/o/l/runs/build")
    expect(action?.prompt).toContain("- test — https://github.com/o/l/runs/test")
    expect(action?.prompt).not.toContain("not listed")
  })

  it("leaves the passing checks out", () => {
    const action = pullRequestHandoff(
      detail({
        checks: { total: 2, passed: 1, failed: 1, pending: 0 },
        checkRuns: [run("build"), run("lint", "passed")],
      }),
    )
    expect(action?.prompt).toContain("- build")
    expect(action?.prompt).not.toContain("lint")
  })

  it("caps the list and says how many it left out", () => {
    const names = Array.from({ length: 11 }, (_, i) => `job-${i}`)
    const prompt = pullRequestHandoff(detail(red(names)))?.prompt ?? ""
    expect(prompt).toContain("- job-7")
    expect(prompt).not.toContain("- job-8")
    expect(prompt).toContain("…and 3 more failing checks not listed.")
  })

  it("falls back to the count when gh reported no runs", () => {
    const action = pullRequestHandoff(
      detail({ checks: { total: 3, passed: 0, failed: 1, pending: 2 }, checkRuns: null }),
    )
    expect(action?.prompt).toContain("1 check is red")
  })

  it("wraps the prompt as one paste, so a newline is not a send", () => {
    const prompt = pullRequestHandoff(detail(red(["build"])))?.prompt ?? ""
    expect(prompt.startsWith("\x1b[200~")).toBe(true)
    expect(prompt.endsWith("\x1b[201~")).toBe(true)
  })
})
