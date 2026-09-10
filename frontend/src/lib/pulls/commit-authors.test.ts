import { describe, expect, it } from "vitest"
import type { PullRequestCommit } from "@/lib/api-types"
import { commitAuthorNotice, commitMetaLine } from "./commit-authors"

const DATE = new Date("2026-07-25T22:32:47Z").toLocaleDateString()

function commit(fields: Partial<PullRequestCommit>): PullRequestCommit {
  return {
    oid: "a4dbc1f",
    headline: "feat: the panel",
    body: "",
    login: "",
    name: "",
    email: "",
    date: "2026-07-25T22:32:47Z",
    ...fields,
  }
}

describe("commitMetaLine", () => {
  it("shows GitHub's own answer as an account", () => {
    expect(commitMetaLine(commit({ login: "omartelo", name: "martelo" }))).toBe(
      `@omartelo committed ${DATE}`,
    )
  })

  // The whole failure: without the mark, this row reads exactly like the one
  // above it, and a commit belonging to nobody reaches a merge unremarked.
  it("marks a commit GitHub could not attribute", () => {
    expect(commitMetaLine(commit({ name: "Ada Lovelace", email: "ada@analytical.example" }))).toBe(
      `Ada Lovelace committed ${DATE} · linked to no GitHub account`,
    )
  })

  // A merge commit from the web flow carries no author at all. Nobody's
  // identity is wrong there, so the mark would be a false alarm.
  it("leaves a commit with no author alone", () => {
    expect(commitMetaLine(commit({}))).toBe(`Committed ${DATE}`)
  })

  it("keeps the line whole when gh reports no date", () => {
    expect(commitMetaLine(commit({ login: "omartelo", date: "" }))).toBe("@omartelo")
    expect(commitMetaLine(commit({ date: "" }))).toBe("")
  })
})

describe("commitAuthorNotice", () => {
  it("stays quiet when every commit is the pull request author's", () => {
    const commits = [commit({ login: "omartelo" }), commit({ oid: "f55ebe9", login: "omartelo" })]
    expect(commitAuthorNotice(commits, "omartelo")).toBe("")
  })

  it("stays quiet with no commits at all", () => {
    expect(commitAuthorNotice(null, "omartelo")).toBe("")
    expect(commitAuthorNotice([], "omartelo")).toBe("")
  })

  // The ceiling this exists for: read by one account, landed under another.
  it("names the account the commits actually landed under", () => {
    const commits = [
      commit({ login: "marcelo-filho_snk" }),
      commit({ oid: "f55ebe9", login: "marcelo-filho_snk" }),
    ]
    expect(commitAuthorNotice(commits, "omartelo")).toBe(
      "2 commits landed under @marcelo-filho_snk, not @omartelo.",
    )
  })

  it("names each stray account once, however many commits it landed", () => {
    const commits = [
      commit({ login: "omartelo" }),
      commit({ oid: "b1", login: "marcelo-filho_snk" }),
      commit({ oid: "b2", login: "marcelo-filho_snk" }),
      commit({ oid: "b3", login: "octocat" }),
    ]
    expect(commitAuthorNotice(commits, "omartelo")).toBe(
      "3 commits landed under @marcelo-filho_snk, @octocat, not @omartelo.",
    )
  })

  it("folds a long list of accounts into a count", () => {
    const commits = ["a", "b", "c", "d", "e"].map((login, i) => commit({ oid: `oid-${i}`, login }))
    expect(commitAuthorNotice(commits, "omartelo")).toBe(
      "5 commits landed under @a, @b, @c and 2 more, not @omartelo.",
    )
  })

  it("names the address no account owns, which is what has to be fixed", () => {
    const commits = [commit({ name: "Ada Lovelace", email: "ada@analytical.example" })]
    expect(commitAuthorNotice(commits, "omartelo")).toBe(
      "1 commit is linked to no GitHub account: ada@analytical.example.",
    )
  })

  it("reports both readings when a branch carries both", () => {
    const commits = [
      commit({ login: "marcelo-filho_snk" }),
      commit({ oid: "b1", name: "Ada", email: "ada@analytical.example" }),
    ]
    expect(commitAuthorNotice(commits, "omartelo")).toBe(
      "1 commit landed under @marcelo-filho_snk, not @omartelo. " +
        "1 commit is linked to no GitHub account: ada@analytical.example.",
    )
  })

  // A commit gh reports with no email at all is still unattributed; the line
  // says so without an empty colon trailing it.
  it("drops the address list when there is no address to name", () => {
    expect(commitAuthorNotice([commit({ name: "Ada" })], "omartelo")).toBe(
      "1 commit is linked to no GitHub account.",
    )
  })

  // Same reason the row leaves it alone: gh reported no author, so there is no
  // identity to call wrong.
  it("never counts an authorless merge commit as unattributed", () => {
    const commits = [commit({ login: "omartelo" }), commit({ oid: "b1" })]
    expect(commitAuthorNotice(commits, "omartelo")).toBe("")
  })

  // gh reports no login for a deleted account. Comparing against "" would call
  // every commit a stray, so the comparison is withheld — the unattributed
  // reading needs no second identity and still stands.
  it("withholds the comparison when the pull request author has no login", () => {
    const commits = [
      commit({ login: "omartelo" }),
      commit({ oid: "b1", name: "Ada", email: "ada@analytical.example" }),
    ]
    expect(commitAuthorNotice(commits, "")).toBe(
      "1 commit is linked to no GitHub account: ada@analytical.example.",
    )
  })
})
