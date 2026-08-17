// Package quota reads the subscription quota a provider's own account API
// reports: the rolling windows a plan is metered in, and how much of each one is
// spent. It is not what a session cost — that is internal/pricing — and not how
// full a context window is — that is internal/terminal's usage readout.
//
// Two of the five providers meter a subscription at all. Claude Code and Codex
// each poll an undocumented endpoint from their own CLI, authenticated with the
// OAuth access token that CLI already wrote to disk; opencode, oh-my-pi and
// Crush run on the user's own API keys, where there is no plan to report.
//
// lich reads those credentials and never writes them. Token rotation belongs to
// the CLI that owns the login: a refresh whose write-back fails spends the
// stored refresh token and logs the user out of their agent, which is a far
// worse outcome than a stale gauge. An expired token is reported as signed out.
package quota

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
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
)

// Window is one metered window of a plan: how much of it is spent, how long it
// runs, and when it starts over.
//
// Label is the window's name as the provider frames it ("Session", "Weekly"), or
// the display name of the model a scoped window belongs to. Seconds is its
// length, so the frontend can print the window it is looking at; 0 when the
// provider does not report one. ResetsAt is RFC 3339, empty when unreported.
type Window struct {
	Label    string `json:"label"`
	Seconds  int    `json:"seconds"`
	Percent  int    `json:"percent"`
	ResetsAt string `json:"resetsAt,omitempty"`
}

// Plan is one provider's quota reading. Provider is a providers.Registry id, so
// the frontend resolves the icon and the login command it already knows.
type Plan struct {
	Provider string   `json:"provider"`
	Name     string   `json:"name"`
	Plan     string   `json:"plan,omitempty"`
	Windows  []Window `json:"windows,omitempty"`
	Status   string   `json:"status"`
}

// Service reads plan quota, caching one reading for every caller.
type Service struct {
	http *http.Client
	// claudeURL and codexURL are the usage endpoints, fields so tests drive the
	// parsers against a local server.
	claudeURL string
	codexURL  string

	// now is time.Now, a field so a test can age the cache without sleeping.
	now func() time.Time

	// mu guards the cached reading and is held across the fetch itself, so a
	// second caller arriving mid-request waits for that answer instead of
	// firing its own at an endpoint that rate-limits.
	mu     sync.Mutex
	cached []Plan
	at     time.Time
}

// New returns a Service pointed at the live endpoints.
func New() *Service {
	return &Service{
		http:      &http.Client{Timeout: httpTimeout},
		claudeURL: claudeUsageURL,
		codexURL:  codexUsageURL,
		now:       time.Now,
	}
}

// Plans is every provider that meters a subscription, in Registry order. There
// is always one entry per such provider — a failed reading is a Status, not an
// omission, so the UI can say which provider it could not read.
func (s *Service) Plans() []Plan {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && s.now().Sub(s.at) < cacheTTL {
		return s.cached
	}
	plans := []Plan{s.claudePlan(), s.codexPlan()}
	s.cached, s.at = plans, s.now()
	return plans
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

// signedOut and failed are the two ways a reading ends without windows. Keeping
// them as constructors keeps every caller's error path one line.
func signedOut(p Plan) Plan {
	p.Status = StatusSignedOut
	return p
}

func failed(p Plan) Plan {
	p.Status = StatusError
	return p
}

// harnessFile locates a file a provider CLI keeps in its own state directory:
// under the directory its environment variable names, else under home. Mirrors
// terminal.harnessDir, which resolves the same two directories for transcripts.
func harnessFile(homeVar, sub string, name ...string) (string, bool) {
	base := os.Getenv(homeVar)
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		base = filepath.Join(home, sub)
	}
	return filepath.Join(append([]string{base}, name...)...), true
}

// readJSON GETs url with the given headers and decodes the body into out. The
// bool is "the endpoint rejected our token": a 401 or 403 is a login the user
// has to redo, which every caller reports differently from a network failure.
func (s *Service) readJSON(url, token string, headers map[string]string, out any) (authOK bool, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return true, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, fmt.Errorf("usage endpoint answered %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return true, fmt.Errorf("usage endpoint answered %d", resp.StatusCode)
	}
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
