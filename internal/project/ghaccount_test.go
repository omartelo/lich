package project

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseGHAccounts(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			"two accounts on one host, active first",
			"github.com\n" +
				"  ✓ Logged in to github.com account omartelo (keyring)\n" +
				"  - Active account: true\n" +
				"  - Token scopes: 'gist', 'read:org', 'repo'\n\n" +
				"  ✓ Logged in to github.com account marcelo-filho_snk (keyring)\n" +
				"  - Active account: false\n",
			[]string{"github.com/omartelo", "github.com/marcelo-filho_snk"},
		},
		{
			// The same login can exist on both hosts, so the host travels with
			// it — and gh needs it back to find the token.
			"a second host contributes its own accounts",
			"github.com\n  ✓ Logged in to github.com account omartelo (keyring)\n" +
				"ghe.example.com\n  ✓ Logged in to ghe.example.com account omartelo (keyring)\n",
			[]string{"github.com/omartelo", "ghe.example.com/omartelo"},
		},
		{"not logged in", "You are not logged into any GitHub hosts.\n", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseGHAccounts(tt.out); !slices.Equal(got, tt.want) {
				t.Errorf("parseGHAccounts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitHubAccounts(t *testing.T) {
	gh := &fakeGH{out: []byte("  ✓ Logged in to github.com account omartelo (keyring)\n")}
	got, err := withGH(gh).GitHubAccounts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got, []string{"github.com/omartelo"}) {
		t.Errorf("GitHubAccounts() = %v, want [github.com/omartelo]", got)
	}
	if want := "auth status"; strings.Join(gh.args, " ") != want {
		t.Errorf("ran %q, want %q", strings.Join(gh.args, " "), want)
	}

	gh = &fakeGH{err: errTest}
	if _, err := withGH(gh).GitHubAccounts(); err == nil {
		t.Error("unauthenticated gh = nil error, want the failure surfaced")
	}
}

// TestAccountTokenReachesGH proves the configured account governs the gh call:
// its token is resolved once and carried by the pull request lookup.
func TestAccountTokenReachesGH(t *testing.T) {
	var seen []string
	gh := &accountGH{token: "gho_secret", pr: `{"number":7,"state":"OPEN"}`, calls: &seen}
	svc := &Service{runner: gh.run}
	svc.SetAccounts(func(string) string { return "github.com/marcelo-filho_snk" })

	pr, err := svc.PullRequestDetail("/repo", 0)
	if err != nil || pr == nil {
		t.Fatalf("PullRequestDetail = (%v, %v), want the PR", pr, err)
	}
	if gh.viewToken != "gho_secret" {
		t.Errorf("pr view ran with token %q, want the configured account's", gh.viewToken)
	}
	if want := "auth token --hostname github.com --user marcelo-filho_snk"; !slices.Contains(seen, want) {
		t.Errorf("calls = %v, want one resolving the account (%q)", seen, want)
	}
}

// TestNoAccountRunsAsActive proves both no-override paths run gh with no token,
// which is gh answering as its own active account.
func TestNoAccountRunsAsActive(t *testing.T) {
	tests := []struct {
		name    string
		lookup  accountLookup
		wantGet int
	}{
		{"no lookup wired", nil, 1},
		{"lookup returns no account", func(string) string { return "" }, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gh := &fakeGH{out: []byte(`{"number":7,"state":"OPEN"}`)}
			svc := &Service{runner: gh.run, accounts: tt.lookup}
			if _, err := svc.PullRequestDetail("/repo", 0); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gh.token != "" {
				t.Errorf("token = %q, want empty (gh's active account)", gh.token)
			}
			if gh.calls != tt.wantGet {
				t.Errorf("gh calls = %d, want %d (no token lookup)", gh.calls, tt.wantGet)
			}
		})
	}
}

// TestAccountTokenFailureFallsBack proves a token that cannot be resolved (gh
// is not authenticated as that account anymore) degrades to the active account
// instead of failing the lookup.
func TestAccountTokenFailureFallsBack(t *testing.T) {
	gh := &accountGH{tokenErr: errTest, pr: `{"number":7,"state":"OPEN"}`}
	svc := &Service{runner: gh.run}
	svc.SetAccounts(func(string) string { return "github.com/gone" })

	pr, err := svc.PullRequestDetail("/repo", 0)
	if err != nil || pr == nil {
		t.Fatalf("PullRequestDetail = (%v, %v), want the PR", pr, err)
	}
	if gh.viewToken != "" {
		t.Errorf("token = %q, want empty after the lookup failed", gh.viewToken)
	}
}

// accountGH answers `auth token` with a token (or an error) and every other
// subcommand with the canned pull request, recording the token each one ran as.
type accountGH struct {
	token     string
	tokenErr  error
	pr        string
	viewToken string
	calls     *[]string
}

func (g *accountGH) run(_ time.Duration, _, token string, args ...string) ([]byte, error) {
	if g.calls != nil {
		*g.calls = append(*g.calls, strings.Join(args, " "))
	}
	if len(args) > 1 && args[0] == "auth" && args[1] == "token" {
		return []byte(g.token + "\n"), g.tokenErr
	}
	g.viewToken = token
	return []byte(g.pr), nil
}

// TestTokenArgs pins how an account is turned back into a gh call. Naming the
// host is what makes an enterprise account resolvable at all: without it gh
// looks the login up on its default host, answers "no oauth token found", and
// tokenFor falls back to the active account without a word.
func TestTokenArgs(t *testing.T) {
	tests := []struct {
		name    string
		account string
		want    []string
	}{
		{
			"an account names its host",
			"github.com/omartelo",
			[]string{"auth", "token", "--hostname", "github.com", "--user", "omartelo"},
		},
		{
			"an enterprise account is looked up on its own host",
			"ghe.example.com/work-login",
			[]string{"auth", "token", "--hostname", "ghe.example.com", "--user", "work-login"},
		},
		{
			// Written before accounts carried a host: gh's default host is
			// where it was found, so that is where it still resolves.
			"a host-less account still resolves",
			"omartelo",
			[]string{"auth", "token", "--user", "omartelo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenArgs(tt.account); !slices.Equal(got, tt.want) {
				t.Errorf("tokenArgs(%q) = %v, want %v", tt.account, got, tt.want)
			}
		})
	}
}
