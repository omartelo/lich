import { describe, expect, it } from "vitest"
import { issueBrief, issueName, parseIssueRef } from "@/lib/issue"
import { toBranchName } from "@/lib/git/branch-name"

describe("parseIssueRef", () => {
  it("reads GitHub's own spelling", () => {
    expect(parseIssueRef("#381")).toBe(381)
    expect(parseIssueRef("  #1  ")).toBe(1)
  })

  it("reads a copied issue URL, with or without a tail", () => {
    expect(parseIssueRef("https://github.com/omartelo/lich/issues/381")).toBe(381)
    expect(parseIssueRef("https://github.com/omartelo/lich/issues/381#issuecomment-9")).toBe(381)
    expect(parseIssueRef("https://ghe.corp.example/team/app/issues/12?foo=1")).toBe(12)
  })

  it("leaves a bare number alone — git takes it as a branch name", () => {
    expect(parseIssueRef("381")).toBeNull()
  })

  it("is not a pull request URL", () => {
    expect(parseIssueRef("https://github.com/omartelo/lich/pull/453")).toBeNull()
  })

  it("names no issue for ordinary branch names and typed tasks", () => {
    expect(parseIssueRef("")).toBeNull()
    expect(parseIssueRef("fix the auth redirect")).toBeNull()
    expect(parseIssueRef("feat/issues-381")).toBeNull()
    expect(parseIssueRef("#")).toBeNull()
    expect(parseIssueRef("#abc")).toBeNull()
    expect(parseIssueRef("#12 and more")).toBeNull()
  })

  it("refuses a number that is not one", () => {
    expect(parseIssueRef("#0")).toBeNull()
    expect(parseIssueRef(`#${"9".repeat(30)}`)).toBeNull()
  })
})

describe("issueName", () => {
  it("puts the number in front, so the branch carries it", () => {
    const issue = {
      number: 381,
      title: "Sandbox backend for Windows (unelevated write protection)",
      body: "",
      url: "",
    }
    expect(issueName(issue)).toBe("381 Sandbox backend for Windows (unelevated write protection)")
    // The name is only ever seen through toBranchName, which is what the dialog
    // prints under the field before anything is created.
    expect(toBranchName(issueName(issue))).toBe("381-sandbox-backend-for-windows")
  })
})

describe("issueBrief", () => {
  const issue = {
    number: 381,
    title: "Sandbox backend for Windows",
    body: "Windows has no user namespace.\n\nMap what an unelevated process can protect.",
    url: "https://github.com/omartelo/lich/issues/381",
  }

  it("is one bracketed paste, so the newlines are text and nothing is sent", () => {
    const brief = issueBrief(issue)
    expect(brief.startsWith("\x1b[200~")).toBe(true)
    expect(brief.endsWith("\x1b[201~")).toBe(true)
    expect(brief).not.toContain("\r")
  })

  it("names the issue, links it, and carries its body", () => {
    const brief = issueBrief(issue)
    expect(brief).toContain("GitHub issue #381 — Sandbox backend for Windows")
    expect(brief).toContain("https://github.com/omartelo/lich/issues/381")
    expect(brief).toContain("Map what an unelevated process can protect.")
  })

  it("leaves no dangling blank lines when the issue has no body", () => {
    const brief = issueBrief({ ...issue, body: "   \n  " })
    expect(brief).toBe(
      "\x1b[200~GitHub issue #381 — Sandbox backend for Windows\n" +
        "https://github.com/omartelo/lich/issues/381\x1b[201~",
    )
  })
})
