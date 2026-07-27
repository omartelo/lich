package project

import (
	"regexp"
	"strings"
	"time"
)

// gh authenticates one active account per host, globally. lich works across
// repositories that answer to different accounts, so a project picks the
// account its gh calls run as: the account is stored per project (the store's
// "vcs.account"), turned into a token here, and passed to every gh call.
//
// An account is "<host>/<login>", because a login only identifies an account
// within one host — the same name can exist on github.com and on an enterprise
// instance, and `gh auth token` looks the login up on the default host unless
// told otherwise. A value with no host is an account stored before this was
// true; it resolves on gh's default host, which is where it was found.
//
// The account only governs what lich asks gh. It does not govern git: a push
// still rides the remote's ssh key and signs with the global user.email.
const ghAccountTimeout = 5 * time.Second

// accountLookup resolves the gh account ("<host>/<login>") for the checkout at
// path, or "" for "whatever account gh has active".
type accountLookup func(path string) string

// SetAccounts wires the per-project account resolution. Left unset, every gh
// call runs as gh's active account — lich's behaviour before this existed.
func (s *Service) SetAccounts(lookup accountLookup) {
	s.accounts = lookup
}

// gh runs one gh subcommand for the checkout at dir, as the account that
// checkout's project selected.
func (s *Service) gh(timeout time.Duration, dir string, args ...string) ([]byte, error) {
	return s.ghFor(dir, timeout, dir, args...)
}

// ghFor runs a gh subcommand inside dir as the account resolved for accountDir.
// The two differ whenever gh has to run inside a checkout the store has never
// seen — a worktree created moments ago has no session row yet, so asking it
// which account to use would answer "none" and silently fall back to gh's
// active one, which is the whole failure this feature exists to prevent.
func (s *Service) ghFor(accountDir string, timeout time.Duration, dir string, args ...string) ([]byte, error) {
	return s.runner(timeout, dir, s.tokenFor(accountDir), args...)
}

// tokenFor resolves the token of the account the checkout's project selected.
// Anything that fails — no project, no account, gh not authenticated as it —
// yields "", which runs the call as gh's active account rather than failing it.
//
// Ceiling: one extra `gh auth token` per gh call. The tokens are short-lived
// and gh owns their refresh, so caching them here would mean owning the
// invalidation too; cache only if the Pulls screen ever feels it.
func (s *Service) tokenFor(dir string) string {
	if s.accounts == nil {
		return ""
	}
	account := s.accounts(dir)
	if account == "" {
		return ""
	}
	out, err := s.runner(ghAccountTimeout, dir, "", tokenArgs(account)...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// tokenArgs builds the `gh auth token` call for one account. The host is named
// whenever the account carries one: without --hostname gh looks the login up on
// its default host only, so an account that lives on an enterprise instance
// would answer "no oauth token found" and fall back to the active account —
// silently, which is the failure this whole feature exists to prevent.
func tokenArgs(account string) []string {
	host, login := splitAccount(account)
	args := []string{"auth", "token"}
	if host != "" {
		args = append(args, "--hostname", host)
	}
	return append(args, "--user", login)
}

// splitAccount reads "<host>/<login>", tolerating the host-less form written
// before accounts carried one.
func splitAccount(account string) (host, login string) {
	if host, login, ok := strings.Cut(account, "/"); ok {
		return host, login
	}
	return "", account
}

// ghAccountLine matches one authenticated account in `gh auth status` output:
// "✓ Logged in to github.com account omartelo (keyring)". Anchored on the
// whole phrase because the same output repeats the word "account" in the
// "Active account: true" line below it. The host is captured too: it is half
// of an account's identity, and gh needs it back to find the token.
var ghAccountLine = regexp.MustCompile(`Logged in to (\S+) account (\S+)`)

// GitHubAccounts lists the accounts gh is authenticated as, "<host>/<login>",
// in gh's own order (each host's active account first). It backs the account
// picker in the project's Version Control settings; an unauthenticated gh
// yields an error, which the picker shows instead of an empty list.
func (s *Service) GitHubAccounts() ([]string, error) {
	out, err := s.runner(ghAccountTimeout, "", "", "auth", "status")
	if err != nil {
		return nil, err
	}
	return parseGHAccounts(string(out)), nil
}

// parseGHAccounts pulls the accounts out of `gh auth status`. Deliberately
// tolerant: an unparseable line is skipped, so a future gh reword degrades to a
// shorter list (the account stays configurable by hand) rather than an error.
func parseGHAccounts(out string) []string {
	matches := ghAccountLine.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		return nil
	}
	accounts := make([]string, 0, len(matches))
	for _, m := range matches {
		accounts = append(accounts, m[1]+"/"+m[2])
	}
	return accounts
}
