// A gh account is "<host>/<login>" (internal/project/ghaccount.go): a login only
// identifies an account within one host, and the same name can exist on
// github.com and on an enterprise instance at once.

/** Splits "<host>/<login>", tolerating the host-less form stored before
 * accounts carried one. The host is "" for those. */
export function splitAccount(account: string): [host: string, login: string] {
  const cut = account.indexOf("/")
  return cut === -1 ? ["", account] : [account.slice(0, cut), account.slice(cut + 1)]
}

/** The host-carrying form of an account stored before hosts travelled with
 * them, or null when it cannot be upgraded without guessing: gh knows no such
 * login, or knows it on more than one host and only the user can say which.
 * Left alone, such a value would sit in the picker beside the same account in
 * its full form — two rows, one account. */
export function upgradeAccount(stored: string, accounts: readonly string[]): string | null {
  if (!stored || stored.includes("/")) {
    return null
  }
  const matches = accounts.filter((account) => splitAccount(account)[1] === stored)
  return matches.length === 1 ? matches[0] : null
}

/** How one account reads in the picker. The host only earns its place when
 * there is more than one to tell apart, or when it is not github.com — on a
 * single-host machine it would be the same word on every row. */
export function accountLabel(account: string, accounts: readonly string[]): string {
  const [host, login] = splitAccount(account)
  if (!host) {
    return login
  }
  const hosts = new Set(accounts.map((a) => splitAccount(a)[0]))
  return hosts.size > 1 || host !== "github.com" ? `${login} — ${host}` : login
}

/** Every row the account picker offers, as the value→label map a Select needs
 * to label its own trigger. Without it the closed trigger stringifies the
 * value, showing the stand-in for "no override" and the host-qualified form
 * of an account rather than the words the rows read. */
export function accountSelectItems(
  activeValue: string,
  activeLabel: string,
  accounts: readonly string[],
): Record<string, string> {
  const items: Record<string, string> = { [activeValue]: activeLabel }
  for (const account of accounts) {
    items[account] = accountLabel(account, accounts)
  }
  return items
}
