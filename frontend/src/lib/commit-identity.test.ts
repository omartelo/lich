import { describe, expect, it } from "vitest"
import { commitIdentityRow } from "./commit-identity"

describe("commitIdentityRow", () => {
  it("names the global identity as the one the account does not govern", () => {
    expect(
      commitIdentityRow({ name: "Jane Doe", email: "jane@example.com", local: false }),
    ).toEqual({
      lead: "Commits land as Jane Doe",
      email: "<jane@example.com>",
      note: "— git user.email, not this account.",
    })
  })

  it("says where a repository-local identity came from", () => {
    expect(
      commitIdentityRow({ name: "Work Jane", email: "jane@work.example", local: true }),
    ).toEqual({
      lead: "Commits land as Work Jane",
      email: "<jane@work.example>",
      note: "— set in this repository, overriding your global one.",
    })
  })

  it("keeps the sentence whole when only the name is missing", () => {
    expect(commitIdentityRow({ name: "", email: "jane@example.com", local: false })).toEqual({
      lead: "Commits land as",
      email: "<jane@example.com>",
      note: "— git user.email, not this account.",
    })
  })

  // The one state worth flagging, and the only one that is not a comparison
  // between the two accounts: git refuses the commit outright.
  it("reports a checkout with no identity at all", () => {
    expect(commitIdentityRow({ name: "", email: "", local: false })).toEqual({
      lead: "No git identity in this checkout.",
      email: "",
      note: "Commits will be refused until user.email is set.",
    })
  })

  it("still reports nothing configured when a name outlives its email", () => {
    const row = commitIdentityRow({ name: "Jane Doe", email: "", local: false })
    expect(row.lead).toBe("No git identity in this checkout.")
    expect(row.email).toBe("")
  })
})
