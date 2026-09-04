// Package quota reads the subscription quota a provider's own account API
// reports: the rolling windows a plan is metered in, and how much of each one is
// spent. It is not what a session cost — that is internal/pricing — and not how
// full a context window is — that is internal/terminal's usage readout.
//
// Two of the six providers are read here. Claude Code and Codex each poll an
// undocumented endpoint from their own CLI, authenticated with the OAuth access
// token that CLI already wrote to disk; opencode, oh-my-pi and Crush run on the
// user's own API keys, where there is no plan to report. Antigravity does meter
// a subscription — a Google account, whose OAuth credentials sit in
// ~/.gemini/oauth_creds.json — but no endpoint of its own has been measured, and
// a gauge built on a guessed one would report a number nobody can check.
//
// Which account is read is a question about one session, never about lich. A
// session can be spawned from a binary the user configured — a wrapper that
// exports its own CLAUDE_CONFIG_DIR, or an OAuth token of its own — and the
// account it spends is then not the one lich's own environment names. So the
// reading is taken against the environment of the process running in that
// session's PTY (Account, wired to internal/terminal). A session lich cannot
// read that from, yet knows runs a binary it did not choose, is reported as
// StatusUnknown: a gauge drawn for the wrong account is worse than no gauge.
//
// That login is also named where the provider will say: Claude's own profile
// route, and the OIDC id token Codex writes beside its access token. Neither is
// a second reading — both ride the cache the windows do. A provider that names
// nobody reports an empty Plan.Account, which is the gauge lich has always
// drawn, not a failure.
//
// lich reads those credentials and never writes them. Token rotation belongs to
// the CLI that owns the login: a refresh whose write-back fails spends the
// stored refresh token and logs the user out of their agent, which is a far
// worse outcome than a stale gauge. An expired token is reported as signed out.
package quota

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/omartelo/lich/internal/providers"
)

const (
	// httpTimeout bounds one usage request. Both endpoints answer in well under
	// a second; this is a hang stop.
	httpTimeout = 10 * time.Second
	// cacheTTL is how long a reading is served before the endpoints are asked
	// again. Both rate-limit aggressively — the Waybar widgets that pioneered
	// these routes recommend polling no tighter than five minutes — and a quota
	// gauge is not a number anyone needs to the second.
	cacheTTL = 5 * time.Minute
)

// Window lifetimes. Anthropic names its windows by kind and never reports how
// long they run, so the two account-wide ones are named here; Codex reports the
// duration itself and is read from the payload.
const (
	sessionWindow = 5 * 60 * 60
	weeklyWindow  = 7 * 24 * 60 * 60
)

// A plan's Status. Anything other than StatusOK carries no windows: the reading
// failed, and the frontend says so rather than drawing an empty gauge.
const (
	StatusOK = "ok"
	// StatusSignedOut is "there is no usable login here" — no credentials file,
	// or the provider rejected the token in it. Both are answered by running the
	// provider's own login, so they are one state, not two.
	StatusSignedOut = "signed-out"
	StatusError     = "error"
	// StatusUnknown is "this session does not spend the account lich can read":
	// it runs a binary the user configured, and lich cannot see the environment
	// that binary set up (no live process, or a platform that exposes no other
	// process's environment). The alternative would be the default account's
	// numbers under a session spending someone else's plan.
	StatusUnknown = "unknown"
)

// redirectVars point Claude at something other than the user's subscription:
// another API host, or a key billed per token and metered by no plan.
var redirectVars = []string{
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
}

// accountVars are the environment variables that decide which account a session
// spends: Claude's own token and config dir, Codex's config dir, and the three
// that redirect Claude away from the subscription.
var accountVars = append([]string{
	claudeTokenVar,
	claudeDirVar,
	codexHomeVar,
}, redirectVars...)

// Window is one metered window of a plan: how much of it is spent, how long it
// runs, and when it starts over.
//
// Label is the window's name as the provider frames it ("Session", "Weekly"), or
// the display name of the model a scoped window belongs to. Seconds is its
// length, so the frontend can print the window it is looking at; 0 when the
// provider does not report one. ResetsAt is RFC 3339, empty when unreported.
// Active is the provider's own verdict on which window is the binding one, when
// it reports one at all. LockedReason is non-empty only when the provider says
// this window cannot be spent past regardless of its percentage.
type Window struct {
	Label        string `json:"label"`
	Seconds      int    `json:"seconds"`
	Percent      int    `json:"percent"`
	ResetsAt     string `json:"resetsAt,omitempty"`
	Active       bool   `json:"active,omitempty"`
	LockedReason string `json:"lockedReason,omitempty"`
}

// Plan is one provider's quota reading. Provider is a providers.Registry id, so
// the frontend resolves the icon and the login command it already knows.
//
// Account names the login the windows were read against — the whole point of
// the custom-binary path being that a session can spend an account lich's own
// environment does not name. It is empty whenever the provider will not say
// which, which is a reading without a name and never an error: a gauge that
// cannot name its account is exactly what lich drew before this field existed.
type Plan struct {
	Provider string   `json:"provider"`
	Name     string   `json:"name"`
	Plan     string   `json:"plan,omitempty"`
	Account  string   `json:"account,omitempty"`
	Windows  []Window `json:"windows,omitempty"`
	Status   string   `json:"status"`
}

// Account is what a lich session's own process says about the account it
// spends. Env is that process's environment — where a wrapper binary puts a
// config dir or a token of its own — and Read is false when lich could not read
// it at all. Custom reports a session spawned from a binary the user
// configured, which is the only case where an unreadable environment is worth
// withholding a reading over: every other session spends the login lich reads.
type Account struct {
	Env    map[string]string
	Custom bool
	Read   bool
}

// hidden reports an account lich cannot identify.
func (a Account) hidden() bool { return a.Custom && !a.Read }

// lookup resolves one account-deciding variable: the session's own process
// wins, else lich's own environment. Every helper that decides which account a
// reading belongs to has to resolve through this one, because a helper reading
// only Env is blind on the two paths that matter most — the machine-wide
// reading Settings asks for carries no session environment, and on macOS and
// Windows no session's environment is readable at all (internal/terminal's
// envReadable). Let two of them disagree and the gauge comes back wrong rather
// than absent: harnessFile finds lich's own credentials file while elsewhere
// cannot see the API key saying that login is never billed.
func (a Account) lookup(name string) string {
	if v := a.Env[name]; v != "" {
		return v
	}
	return os.Getenv(name)
}

// elsewhere reports an account pointed at something other than the user's own
// subscription. Reading lich's own login for one of those would draw a gauge
// for an account nobody is spending.
func (a Account) elsewhere() bool {
	for _, name := range redirectVars {
		if a.lookup(name) != "" {
			return true
		}
	}
	return false
}

// Sessions answers what a session's process says about the account it spends.
// main wires it to the terminal service; a Service without one reads only
// lich's own environment, which is the machine-wide question Settings asks.
type Sessions func(sessionID string) Account

// Service reads plan quota, caching one reading per account.
type Service struct {
	http *http.Client
	// claudeURL, codexURL, probeURL and profileURL are the endpoints, fields so
	// tests drive the parsers against a local server.
	claudeURL  string
	codexURL   string
	probeURL   string
	profileURL string

	// now is time.Now, a field so a test can age the cache without sleeping.
	now func() time.Time

	// sessions resolves a session id to the account it spends; nil until main
	// wires it, and never called for the machine-wide reading.
	sessions Sessions

	// mu guards the cache and is held across the fetch itself, so a second
	// caller arriving mid-request waits for that answer instead of firing its
	// own at an endpoint that rate-limits.
	mu    sync.Mutex
	cache map[string]reading
}

// reading is one cached answer, and when it was taken.
type reading struct {
	plans []Plan
	at    time.Time
}

// New returns a Service pointed at the live endpoints.
func New() *Service {
	return &Service{
		http:       &http.Client{Timeout: httpTimeout},
		claudeURL:  claudeUsageURL,
		codexURL:   codexUsageURL,
		probeURL:   claudeProbeURL,
		profileURL: claudeProfileURL,
		now:        time.Now,
		cache:      make(map[string]reading),
	}
}

// SetSessions wires the answer to "which account does this session spend".
// Startup wiring, called once before the service serves.
func (s *Service) SetSessions(fn Sessions) { s.sessions = fn }

// Plans is every provider that meters a subscription, in Registry order, read
// for the account sessionID spends. There is always one entry per such provider
// — a failed reading is a Status, not an omission, so the UI can say which
// provider it could not read.
//
// An empty sessionID reads lich's own environment: the Settings screen asks
// about the machine, not about a card.
func (s *Service) Plans(sessionID string) []Plan {
	account := Account{Read: true}
	if sessionID != "" && s.sessions != nil {
		account = s.sessions(sessionID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := cacheKey(account)
	if cached, ok := s.cache[key]; ok && s.now().Sub(cached.at) < cacheTTL {
		return cached.plans
	}
	plans := []Plan{s.claudePlan(account), s.codexPlan(account)}
	s.cache[key] = reading{plans: plans, at: s.now()}
	return plans
}

// cacheKey identifies the account a reading belongs to, so two sessions on the
// same login share one reading — and a session on another login gets its own
// instead of being served someone else's numbers. Every account lich cannot
// identify shares the one key whose reading needs no request at all.
//
// It keys on the session's own values alone, not on lookup: the fallback half
// of lookup is lich's own environment, one constant for every account in the
// process, so folding it in would lengthen every key without telling one more
// pair of accounts apart. The account naming none of these — the machine-wide
// reading, and every session on a platform whose environment lich cannot read
// — keys empty, which is what puts it and Settings on one reading.
func cacheKey(a Account) string {
	if a.hidden() {
		return "?"
	}
	values := make([]string, 0, len(accountVars))
	for _, name := range accountVars {
		values = append(values, a.Env[name])
	}
	return strings.Join(values, "\x00")
}

// plan is the common shape of a reading, before its windows are known.
func plan(id string) Plan {
	name := id
	for _, p := range providers.Registry {
		if p.ID == id {
			name = p.Name
		}
	}
	return Plan{Provider: id, Name: name, Status: StatusOK}
}

// signedOut, failed and unknown are the three ways a reading ends without
// windows. Keeping them as constructors keeps every caller's error path one
// line.
func signedOut(p Plan) Plan {
	p.Status = StatusSignedOut
	return p
}

func failed(p Plan) Plan {
	p.Status = StatusError
	return p
}

func unknown(p Plan) Plan {
	p.Status = StatusUnknown
	return p
}

// harnessFile locates a file a provider CLI keeps in its own state directory:
// under the directory its environment variable names, else under home. The
// session's own environment wins over lich's — a wrapper binary exporting
// another config dir is the whole reason this is not a plain os.Getenv. Mirrors
// terminal.harnessDir, which resolves the same two directories for transcripts.
func harnessFile(a Account, homeVar, sub string, name ...string) (string, bool) {
	base := a.lookup(homeVar)
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		base = filepath.Join(home, sub)
	}
	return filepath.Join(append([]string{base}, name...)...), true
}

// send performs one request with the given bearer token and headers, and hands
// back the response for the caller to read. The bool is "the endpoint rejected
// our token": a 401 or 403 is a login the user has to redo, which every caller
// reports differently from a network failure. The body is closed here on every
// path that returns no response.
func (s *Service) send(method, url, token, body string, headers map[string]string) (*http.Response, bool, error) {
	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, true, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		_ = resp.Body.Close()
		return nil, false, fmt.Errorf("usage endpoint answered %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, true, fmt.Errorf("usage endpoint answered %d", resp.StatusCode)
	}
	return resp, true, nil
}

// readJSON GETs url with the given headers and decodes the body into out.
func (s *Service) readJSON(url, token string, headers map[string]string, out any) (authOK bool, err error) {
	resp, authOK, err := s.send(http.MethodGet, url, token, "", headers)
	if err != nil {
		return authOK, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return true, fmt.Errorf("decode usage response: %w", err)
	}
	return true, nil
}

// readCredentials decodes the JSON credentials file at path into out. The bool
// is false for every reason there is no login to read with — the file is
// absent, unreadable, or not the JSON it should be.
func readCredentials(path string, out any) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, out) == nil
}

// percent clamps a reported share to the 0–100 the gauges are drawn on. A
// provider that reports 103% of a soft limit is at the top of the bar, not off
// the end of it.
func percent(v float64) int {
	if math.IsNaN(v) || v <= 0 {
		return 0
	}
	if v >= 100 {
		return 100
	}
	return int(math.Round(v))
}

// title upper-cases a plan name the wire spells in lower case ("max" → "Max").
func title(s string) string {
	if s == "" {
		return ""
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}
