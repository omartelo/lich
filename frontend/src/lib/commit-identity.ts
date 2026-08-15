import type { CommitIdentity } from "./api-types"

/** The three parts of the identity row, so the email can be set in monospace
 * without the rest of the sentence being assembled in JSX. */
export interface CommitIdentityRow {
  lead: string
  /** "<jane@example.com>"; "" when the checkout has no identity to name. */
  email: string
  note: string
}

/** How a checkout's commit identity reads under the account picker. The two
 * facts sit side by side and nothing here compares them: noreply forms, vanity
 * domains and org aliases make an email-to-login check a false-positive farm,
 * and the person reading owns the conclusion. The one state that is not a
 * comparison — no identity at all — is a fact about the checkout, and git will
 * refuse the commit for it. */
export function commitIdentityRow(identity: CommitIdentity): CommitIdentityRow {
  if (!identity.email) {
    return {
      lead: "No git identity in this checkout.",
      email: "",
      note: "Commits will be refused until user.email is set.",
    }
  }
  return {
    lead: identity.name ? `Commits land as ${identity.name}` : "Commits land as",
    email: `<${identity.email}>`,
    note: identity.local
      ? "— set in this repository, overriding your global one."
      : "— git user.email, not this account.",
  }
}
